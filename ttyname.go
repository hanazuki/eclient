package main

// #include <unistd.h>
import "C"

func TtyName(fd uintptr) (string, error) {
	p, err := C.ttyname(C.int(fd))
	if err != nil {
		return "", err
	}
	return C.GoString(p), nil
}
