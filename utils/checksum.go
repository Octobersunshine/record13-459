package checksum

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"strings"
)

type ChecksumType string

const (
	TypeMD5    ChecksumType = "md5"
	TypeSHA256 ChecksumType = "sha256"
)

type ChecksumResult struct {
	Type  ChecksumType `json:"type"`
	Value string       `json:"value"`
}

type VerifyResult struct {
	Matched bool   `json:"matched"`
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
}

type RestoreVerifyResult struct {
	Success      bool   `json:"success"`
	Readable     bool   `json:"readable"`
	SizeMatch    bool   `json:"size_match"`
	FormatValid  bool   `json:"format_valid"`
	ErrorMessage string `json:"error_message,omitempty"`
}

func Calculate(filePath string, checksumType ChecksumType) (*ChecksumResult, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %w", err)
	}
	defer file.Close()

	var hasher hash.Hash
	switch strings.ToLower(string(checksumType)) {
	case string(TypeMD5):
		hasher = md5.New()
	case string(TypeSHA256):
		hasher = sha256.New()
	default:
		return nil, errors.New("不支持的校验和类型")
	}

	if _, err := io.Copy(hasher, file); err != nil {
		return nil, fmt.Errorf("计算校验和失败: %w", err)
	}

	return &ChecksumResult{
		Type:  checksumType,
		Value: hex.EncodeToString(hasher.Sum(nil)),
	}, nil
}

func CalculateMD5(filePath string) (*ChecksumResult, error) {
	return Calculate(filePath, TypeMD5)
}

func CalculateSHA256(filePath string) (*ChecksumResult, error) {
	return Calculate(filePath, TypeSHA256)
}

func Verify(filePath string, checksumType ChecksumType, expected string) (*VerifyResult, error) {
	result, err := Calculate(filePath, checksumType)
	if err != nil {
		return nil, err
	}

	matched := strings.EqualFold(result.Value, strings.TrimSpace(expected))
	return &VerifyResult{
		Matched:  matched,
		Expected: expected,
		Actual:   result.Value,
	}, nil
}

func VerifyRestore(filePath string, expectedSize int64) (*RestoreVerifyResult, error) {
	result := &RestoreVerifyResult{}

	file, err := os.Open(filePath)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("无法打开文件: %v", err)
		return result, nil
	}
	defer file.Close()

	result.Readable = true

	fileInfo, err := file.Stat()
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("获取文件信息失败: %v", err)
		return result, nil
	}

	result.SizeMatch = fileInfo.Size() == expectedSize
	if !result.SizeMatch {
		result.ErrorMessage = fmt.Sprintf("文件大小不匹配: 期望 %d 字节, 实际 %d 字节", expectedSize, fileInfo.Size())
		return result, nil
	}

	header := make([]byte, 8)
	n, err := file.Read(header)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("读取文件头失败: %v", err)
		return result, nil
	}

	result.FormatValid = isValidBackupFormat(header[:n])
	if !result.FormatValid {
		result.ErrorMessage = "文件格式无效，疑似损坏"
		return result, nil
	}

	if !checkFileIntegrity(file) {
		result.ErrorMessage = "文件内容校验失败，存在损坏区域"
		return result, nil
	}

	result.Success = true
	return result, nil
}

func isValidBackupFormat(header []byte) bool {
	if len(header) < 4 {
		return false
	}

	magicNumbers := [][]byte{
		{0x50, 0x4B, 0x03, 0x04},
		{0x1F, 0x8B, 0x08},
		{0x42, 0x5A, 0x68},
		{0x75, 0x73, 0x74, 0x61},
		{0x2D, 0x2D, 0x20, 0x50},
		{0x53, 0x51, 0x4C},
	}

	for _, magic := range magicNumbers {
		if len(header) >= len(magic) {
			match := true
			for i, b := range magic {
				if header[i] != b {
					match = false
					break
				}
			}
			if match {
				return true
			}
		}
	}

	if len(header) >= 4 {
		if (header[0] >= 0x20 && header[0] <= 0x7E) &&
			(header[1] >= 0x20 && header[1] <= 0x7E) &&
			(header[2] >= 0x20 && header[2] <= 0x7E) &&
			(header[3] >= 0x20 && header[3] <= 0x7E) {
			return true
		}
	}

	return false
}

func checkFileIntegrity(file *os.File) bool {
	_, err := file.Seek(0, io.SeekStart)
	if err != nil {
		return false
	}

	buf := make([]byte, 64*1024)
	totalRead := int64(0)
	for {
		n, err := file.Read(buf)
		if n > 0 {
			totalRead += int64(n)
			for i := 0; i < n; i++ {
				if buf[i] == 0x00 {
					zeroCount := 1
					for j := i + 1; j < n && j < i+1024; j++ {
						if buf[j] == 0x00 {
							zeroCount++
						} else {
							break
						}
					}
					if zeroCount > 512 {
						return false
					}
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return false
		}
	}

	return totalRead > 0
}
