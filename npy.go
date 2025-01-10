package main

import (
	"encoding/json"
	"fmt"
	"golang.org/x/text/encoding/unicode/utf32"
	"log"
	"os"
	"strings"
)

func ReadNpyStrArray(file *os.File) ([]string, error) {
	file.Seek(0, 0)
	// check the magic number (0x93)NUMPY
	magic := make([]byte, 6)
	_, err := file.Read(magic)
	if err != nil {
		return nil, err
	}
	if magic[0] != 0x93 {
		return nil, fmt.Errorf("invalid magic number: %v", magic)
	}
	if string(magic[1:]) != "NUMPY" {
		return nil, fmt.Errorf("invalid magic string: %v", magic)
	}
	version := make([]byte, 2)
	_, err = file.Read(version)
	if err != nil {
		return nil, err
	}
	majorVersion := version[0]
	minorVersion := version[1]

	log.Printf("Reading numpy version %d.%d file", majorVersion, minorVersion)

	// read the header
	headerLen := make([]byte, 2)
	_, err = file.Read(headerLen)
	if err != nil {
		return nil, err
	}
	headerLenInt := int(headerLen[0]) + int(headerLen[1])*256
	headerByte := make([]byte, headerLenInt)

	_, err = file.Read(headerByte)
	if err != nil {
		return nil, err
	}

	// read the data
	// {'descr': '<U32', 'fortran_order': False, 'shape': (?,), }
	// python dict to json
	pythonDictStr := string(headerByte)
	jsonDictStr := strings.Replace(pythonDictStr, "'", "\"", -1)
	jsonDictStr = strings.Replace(jsonDictStr, "False", "false", -1)
	jsonDictStr = strings.Replace(jsonDictStr, "True", "true", -1)
	jsonDictStr = strings.Replace(jsonDictStr, "None", "null", -1)
	jsonDictStr = strings.Replace(jsonDictStr, "(", "[", -1)
	jsonDictStr = strings.Replace(jsonDictStr, ",),", "]", -1)

	var header struct {
		Describer    string `json:"descr"`
		FortranOrder bool   `json:"fortran_order"`
		Shape        []int  `json:"shape"`
	}
	err = json.Unmarshal([]byte(jsonDictStr), &header)
	if err != nil {
		return nil, err
	}

	// fixed size string
	if length, match := matchFixedLenStr(header.Describer); match {
		results := make([]string, header.Shape[0])
		u := utf32.UTF32(utf32.LittleEndian, utf32.IgnoreBOM)
		decoder := u.NewDecoder()
		for i := 0; i < header.Shape[0]; i++ {
			strByte := make([]byte, 4*length)
			_, err = file.Read(strByte)
			if err != nil {
				return nil, err
			}
			// UTF-32
			strByte, err = decoder.Bytes(strByte)
			if err != nil {
				return nil, err
			}
			results[i] = strings.Trim(string(strByte), "\000")
		}
		return results, nil
	}

	// read the data
	if header.Describer == "<U32" {
		// 32 characters per string
	}
	return nil, fmt.Errorf("unsupported describer: %v", header.Describer)
}
