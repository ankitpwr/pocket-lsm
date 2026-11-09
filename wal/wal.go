package wal

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type WAL struct {
	directory      string
	lastSequenceNo uint64
	bufWriter      *bufio.Writer
	maxFileSize    int64
	maxSeg         int
	currentSegIdx  int
	currentSeg     *os.File
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

	//why open file again and seek to end of the file?

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
