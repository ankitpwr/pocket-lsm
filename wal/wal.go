package wal

import (
	"bufio"
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

func NewWal(directory string, maxSeg int, maxFileSize int64) (*WAL, error) {
	//create directory if if does not exits
	if err := os.MkdirAll(directory, 0755); err != nil {
		return nil, err
	}

	//search for all the logs files
	files, err := filepath.Glob(filepath.Join(directory, "segment-"+"*"))
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
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644) // why os append, create flag needed here?
	if err != nil {
		return nil, err
	}

	wal := &WAL{
		directory:      directory,
		maxSeg:         maxSeg,
		maxFileSize:    maxFileSize,
		currentSegIdx:  lastSegId,
		currentSeg:     file, //why not *file?
		bufWriter:      bufio.NewWriter(file),
		lastSequenceNo: 0,
	}

	return wal, nil

}

func (wal *WAL) writeEntry(data []byte) error {
	wal.mu.Lock()
	defer wal.mu.Unlock()

	//create new segment if current exceed
	if err := wal.rotateSegIfNeeded(); err != nil {
		return err
	}

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
		return nil
	}
	return nil
}
