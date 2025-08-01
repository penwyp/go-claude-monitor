package util

import (
	"fmt"
	"hash/crc32"
	"io"
	"os"
)

// CalculateFileFingerprint calculates CRC32 fingerprint of the last 2KB of a file
func CalculateFileFingerprint(filepath string) (string, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return "", err
	}

	size := stat.Size()
	readSize := int64(2048)
	if size < readSize {
		readSize = size
	}

	// Seek to 2KB before end of file
	_, err = file.Seek(-readSize, io.SeekEnd)
	if err != nil {
		return "", err
	}

	data := make([]byte, readSize)
	_, err = file.Read(data)
	if err != nil {
		return "", err
	}

	crc := crc32.ChecksumIEEE(data)
	return fmt.Sprintf("%08x", crc), nil
}
