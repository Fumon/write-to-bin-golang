package main

import (
	"fmt"
	"strconv"
	"os"
	"os/exec"
	"path/filepath"
	"io"
	"io/ioutil"
	"bytes"
	"encoding/binary"
	"unsafe"
)

// Magic footer bytes.
var magicbits [4]byte = [4]byte{byte(200), byte(76), byte(112), byte(0)}

func main() {

	// Initial behaviour, real file with possible data attached.
	if x := len(os.Args); x == 1 {
		// Get an absolute path
		path_to_this := GetAbsPath()

		// Open ourselves.
		this_file, err := os.Open(path_to_this)
		if err != nil {
			panic(fmt.Sprintln("Could not open self:\n\t", err))
		}
		defer this_file.Close()

		// Transfer binary to temporary file and point it at this one
		subvert_the_pager(this_file)

		// Die as quickly as possible.
		os.Exit(0)
	} else if os.Args[1] != "run" {
		// Run flag parsing
	}

	// We are the temporary file and must now proceed to open the original.
	inb, err := strconv.ParseInt(os.Args[1], 10, 0)
	if err != nil {
		panic("Cannot parse pid of starter")
	}
	pid_of_starter := int(inb)
	orig_path := os.Args[2]

	// Wait on the original thread to stop so we can open the file.
	starter_process, err := os.FindProcess(pid_of_starter)
	if err != nil {
		// TODO: Handle case where process is still alive.
	} else {
		starter_process.Wait()
	}
	

	// Now open ourselves
	orig_file, err := os.OpenFile(orig_path, os.O_RDWR, os.ModePerm)
	if err != nil {
		panic(fmt.Sprintln("Could not open file: ", err))
	}
	defer orig_file.Close()
	

	if !CheckFooter(orig_file){
		fmt.Println("Not magificated, attempting.")
		Magificate(orig_file)
		fmt.Println("Successfully magificated: ", orig_path)
	} else {
		fmt.Println("original is magificated")
	}

}

// Gets absolute path to the executable so long as the working directory hasn't changed
func GetAbsPath() (estpath string){
	targ := os.Args[0]
	if filepath.IsAbs(targ) {
		estpath = targ
	}else {
		wd, _ := os.Getwd()
		estpath = filepath.Clean(filepath.Join(wd, targ))
	}
	return
}


// Copies the binary to temp and runs from there, modifying the original file.
func subvert_the_pager(f *os.File) {
	// Hold an end address which defines the last byte of the binary
	var Bin_last_byte int64 = 0
	var err error = nil

	Bin_last_byte, err = FindLastBinByte(f)

	// Create temporary file
	// Get name of bin for prefix
	name := filepath.Base(f.Name())
	tmpfile, err := ioutil.TempFile("", name)
	if err != nil {
		panic(fmt.Sprintln("Could not create temp file:\n\t", err))
	}

	// Now copy the binary into the temporary file.
	f.Seek(0, os.SEEK_SET)
	_, err = io.CopyN(tmpfile, f, Bin_last_byte)
	if err != nil {
		panic(fmt.Sprintln("Problem while copying binary:\n\t", err))
	}

	// Save the name of the temp file.
	tmpname := tmpfile.Name()
	// Change the file permissions on the temp file so we can execute it.
	os.Chmod(tmpname, 0700)

	// Sync and then close the temp file..
	tmpfile.Sync()
	tmpfile.Close()

	// Now launch the program with the path to the original file.
	pid := string(os.Getpid())

	cmd := exec.Command(tmpname, pid, f.Name())
	// Try to bind stdout
	cmd.Stdout = os.Stdout
	cmd.Start() // Don't wait
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

// Takes a read only file and returns a boolean stating if it is a FuuFile
// Determines by checking the footer bytes.
func CheckFooter(file *os.File) bool {
	// Seek 4 bytes back from the end
	file.Seek(-4, os.SEEK_END)

	// Check the file ending for compatibility
	magicbuff := make([]byte, 4)
	if _, err := file.Read(magicbuff); err != nil {
		panic("Unable to read magic bits")
	}

	return (bytes.Compare(magicbuff, magicbits[:]) == 0)
}

// Takes a file and reads its FuuFooter bytes.
// Returns a last binary byte and  an error variable.
// The last binary byte will either be the value contained in the FuuFooter
// or the last byte in the file if it is not initialized
func FindLastBinByte(f *os.File) (int64, error) {
	if !CheckFooter(f) {
		// Seek to last byte
		b, _ := f.Seek(0, os.SEEK_END)
		return b, nil
	}

	// We have a FuuFile, parse the footer.
	footer := &FuuFoot{}

	f.Seek(-1 * int64(FuuFootSize), os.SEEK_END)
	err := binary.Read(f, binary.LittleEndian, footer)
	if err != nil {
		panic("Error reading footer")
	}

	return footer.Bin_end, nil
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