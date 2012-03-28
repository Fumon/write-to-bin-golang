package main

import (
	"fmt"
	"os"
	"bytes"
	"encoding/binary"
	"unsafe"
)

var magicbits [4]byte = [4]byte{byte(200), byte(76), byte(112), byte(0)}


func main() {
	defer func() {
		fmt.Println("Done")
		if x := recover(); x != nil {
			fmt.Println("Paniced with runtime error \n", x)
			for {
				_ = 1+1
			}
		}
	}()
	// Get our filename
	path_to_this := os.Args[0]
	
	// Now open ourselves
	this_file, err := os.OpenFile(path_to_this, os.O_RDWR, os.ModePerm)
	//this_file, err := os.Open(path_to_this)
	if err != nil {
		panic(fmt.Sprintln("Could not open file: ", err))
	}
	defer this_file.Close()

	// Seek 4 bytes back from the end
	this_file.Seek(-4, 2)

	// Check the file ending for compatibility
	magicbuff := make([]byte, 4)
	if _, err := this_file.Read(magicbuff); err != nil {
		panic("Unable to read magic bits")
	}

	magiced := true
	if bytes.Compare(magicbuff, magicbits[:]) != 0 {
		//panic(fmt.Sprintf("Incorrect magic bits, got: %X %X %X %X", magicbuff[0], magicbuff[1], magicbuff[2], magicbuff[3]))
		magiced = false
	}

	if !magiced {
		Magificate(this_file)
	}

}

type FuuFoot struct {
	// In the order which it would appear at the end of the file
	Bin_end int64
	// Version byte
	Version byte
	// Magicbytes
	magic [4]byte
}

// Constant
var FuuFootSize = unsafe.Sizeof(FuuFoot{})


// Generate a FuuFoot version one from the info in FuuFile
func GenFuuFoot1(file *FuuFile) *FuuFoot {
	return &FuuFoot{file.Bin_last_byte, byte(1), magicbits}
}

func (rec *FuuFoot) Output() (b []byte, err error) {
	outbuff := new(bytes.Buffer)
	var data = []interface{}{
		rec.Bin_end,
		rec.Version,
		rec.magic,
	}
	for _, v := range data {
		err = binary.Write(outbuff, binary.LittleEndian, v)
		if err != nil {
			b = nil
			return
		}
	}
	b = outbuff.Bytes()

	return
}

type FuuFile struct {
	*os.File
	magicked		bool

	// Size of binary at start (also the offset to Fuuoptions once magicked)
	Bin_last_byte		int64

	// The current end of file as an address (can change as needed)
	// TODO: Reclaim size as needed
	End_of_file 		int64
	// End of data, should be End_of_file-footersize
	End_of_data		int64
}

// TODO fuufile init
// func InitFuuFile(f *os.File) {}

// Writes a new footer on the existing file
func (rec *FuuFile) ReFoot() (err error) {
	// Start from scratch
	footbytes, err := GenFuuFoot1(rec).Output()
	if err != nil {
		return
	}

	if rec.magicked {
		// Seek to the beginning of the footer
		rec.Seek(rec.End_of_data, os.SEEK_SET)
	} else {
		rec.Seek(rec.End_of_file, os.SEEK_SET)
		defer func() {
			if err == nil {
				rec.magicked = true
			}
		}()
	}

	_, err = rec.Write(footbytes)
	if err != nil {
		return
	}
	err = nil
	return
}



// Takes a regular file and makes a FuuFile
func Magificate(this_file *os.File) (f *FuuFile, err error) {
	// Get the size of the file at seek -1 (skip back from the null bit)
	end_file_offset, err := this_file.Seek(0, 2)
	if err != nil {
		panic("Problem seeking to end of file")
	}

	// Initialize the struct
	f = &FuuFile{this_file, false, end_file_offset, end_file_offset, end_file_offset}

	if err := f.ReFoot(); err != nil {
		panic("Issues writing to file")
	}

	err = nil
	return
}