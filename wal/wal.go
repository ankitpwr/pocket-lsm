package wal

import (
	"bufio"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

type WAL struct {
	directory      string
	lastSequenceNo uint64
	bufWriter      *bufio.Writer
	maxFileSize    int64
	maxSeg         int
	currentSegIdx  int
	currentSeg     *os.File
	mu             sync.Mutex
}

type WAL_Entry struct {
	logsequenceNo uint64
	data          []byte
	CRC           uint32
	isCheckPoint  bool
}

func NewWal(directory string, maxSeg int, maxFileSize int64) (*WAL, error) {
	//create directory if if does not exits
	if err := os.MkdirAll(directory, 0755); err != nil {
		return nil, err
	}

	//search for all the logs files
	files, err := filepath.Glob(filepath.Join(directory, "segment-*"))
	if err != nil {
		return nil, err
	}

	var lastSegId int
	if len(files) > 0 {
		segIdx, err := findLastSegIdx(files)
		if err != nil {
			return nil, err
		}
		lastSegId = segIdx
	} else {
		_, err := createSegmentFile(directory, 0)
		if err != nil {
			return nil, err
		}

		//why to close file now?
	}

	filePath := filepath.Join(directory, "segment-"+strconv.Itoa(lastSegId))
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644) // why os append, create flag needed here as we already have created the file?
	if err != nil {
		return nil, err
	}

	wal := &WAL{
		directory:      directory,
		maxSeg:         maxSeg,
		maxFileSize:    maxFileSize,
		currentSegIdx:  lastSegId,
		currentSeg:     file,
		bufWriter:      bufio.NewWriter(file),
		lastSequenceNo: 0,
	}

	return wal, nil

}

func (wal *WAL) writeEntry(data []byte, isCheckPoint bool) error {
	wal.mu.Lock()
	defer wal.mu.Unlock()

	//create new segment if current exceed
	if err := wal.rotateSegIfNeeded(); err != nil {
		return err
	}

	wal.lastSequenceNo++

	//if checkpoint add it

	crc := crc32.NewIEEE()
	var lsnBuf [8]byte
	binary.LittleEndian.PutUint64(lsnBuf[:], wal.lastSequenceNo) //Split the uint64 lastSequenceNo into 8 individual bytes
	//feed 8-bytes and data to CRC calculator
	crc.Write(lsnBuf[:])
	crc.Write(data)

	entry := &WAL_Entry{
		logsequenceNo: wal.lastSequenceNo,
		data:          data,
		CRC:           crc.Sum32(), //produce 4 byte check-sum
		isCheckPoint:  isCheckPoint,
	}

	//why to fsync when check point is true
	if isCheckPoint {
		if err := wal.sync(); err != nil {
			return err
		}
	}
	return wal.writeToBuffer(entry)
}

func (wal *WAL) writeToBuffer(entry *WAL_Entry) error {
	//Allocate a 16-byte slice for the fixed-size header(crc + lsn + size_of_the_data)
	header := make([]byte, 17)
	binary.LittleEndian.PutUint32(header[0:4], entry.CRC)
	binary.LittleEndian.PutUint64(header[4:12], entry.logsequenceNo)
	binary.LittleEndian.PutUint32(header[12:16], uint32(len(entry.data)))

	//why doent use little endian put here?
	if entry.isCheckPoint {
		header[16] = 1
	} else {
		header[16] = 0
	}

	//Copy the 16-byte header into buffer
	if _, err := wal.bufWriter.Write(header); err != nil {
		return err
	}
	//Copy data bytes into buffer
	if _, err := wal.bufWriter.Write(entry.data); err != nil {
		return err
	}
	return nil
}

func (wal *WAL) readAll(readFromCheckpoint bool) ([]*WAL_Entry, error) {

	wal.mu.Lock()
	defer wal.mu.Unlock()

	if err := wal.sync(); err != nil {
		return nil, err
	}

	file, err := os.OpenFile(wal.currentSeg.Name(), os.O_RDONLY, 0644)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return readAllEntriesFromFile(file, readFromCheckpoint)
}

func readAllEntriesFromFile(file *os.File, readFromCheckpoint bool) ([]*WAL_Entry, error) {
	var entries []*WAL_Entry
	header := make([]byte, 17)

	for {
		//Read the 17-byte fixed header
		if _, err := io.ReadFull(file, header); err != nil {

			//cleanly reached end of file (no more entries)
			if errors.Is(err, io.EOF) {
				break
			}
			return entries, nil
		}

		crc := binary.LittleEndian.Uint32(header[0:4])
		lsn := binary.LittleEndian.Uint64(header[4:12])
		dataSize := binary.LittleEndian.Uint32(header[12:16])
		isCheckpoint := header[16] == 1

		data := make([]byte, dataSize)

		//will it read from where we left?
		if _, err := io.ReadFull(file, data); err != nil {
			//entries ended halfway
			return entries, err
		}

		entry := &WAL_Entry{
			logsequenceNo: lsn,
			data:          data,
			CRC:           crc,
			isCheckPoint:  isCheckpoint,
		}

		if !verifyCRC(entry) {
			return entries, errors.New("wal: corrupted entry detected (CRC mismatch)")
		}

		// only entries after last checkpoint
		if readFromCheckpoint && isCheckpoint {
			entries = nil
			continue
		}
		entries = append(entries, entry)

	}
	return entries, nil

}

// recalculate crc checksum and compare it with entry CRC
func verifyCRC(entry *WAL_Entry) bool {
	crc := crc32.NewIEEE()

	var lsnBuf [8]byte
	binary.LittleEndian.PutUint64(lsnBuf[:], entry.logsequenceNo)
	crc.Write(lsnBuf[:])
	crc.Write(entry.data)

	actualCRC := crc.Sum32()
	return actualCRC == entry.CRC
}

// finds the highest segment ID from a list of file paths
func findLastSegIdx(files []string) (int, error) {
	var lastSegId int

	for _, file := range files {
		_, fileName := filepath.Split(file)
		segIdx, err := strconv.Atoi(strings.TrimPrefix(fileName, "segment-"))
		if err != nil {
			return 0, err
		}

		if segIdx > lastSegId {
			lastSegId = segIdx
		}
	}
	return lastSegId, nil
}

// create new segment file
func createSegmentFile(directory string, segmentId int) (*os.File, error) {
	filePath := filepath.Join(directory, "segment-"+strconv.Itoa(segmentId))
	file, err := os.Create(filePath)
	if err != nil {
		return nil, err
	}
	return file, nil
}

// check if rotation is required
func (wal *WAL) rotateSegIfNeeded() error {
	fileStats, err := wal.currentSeg.Stat()
	if err != nil {
		return err
	}
	//returns the number of bytes
	if fileStats.Size()+int64(wal.bufWriter.Buffered()) >= wal.maxFileSize {
		if err := wal.rotateSegment(); err != nil {
			return err
		}
	}
	return nil
}

// create new log segment file
func (wal *WAL) rotateSegment() error {
	if err := wal.sync(); err != nil {
		return err
	}

	if err := wal.currentSeg.Close(); err != nil {
		return err
	}

	wal.currentSegIdx++
	newFile, err := createSegmentFile(wal.directory, wal.currentSegIdx)
	if err != nil {
		return err
	}
	//delete old segment if segment couts > maxsegments
	wal.currentSeg = newFile
	wal.bufWriter = bufio.NewWriter(newFile)
	return nil
}

func (wal *WAL) sync() error {
	// explicitly write any accumulated in memory to the file
	if err := wal.bufWriter.Flush(); err != nil {
		return err
	}

	//fsync to disk
	if err := wal.currentSeg.Sync(); err != nil {
		return err
	}
	return nil
}
