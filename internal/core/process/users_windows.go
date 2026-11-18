//go:build windows

package process

import (
	"context"
	"errors"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// usersOf lists the processes whose executable or current directory is inside one of the roots. The
// executable comes from QueryFullProcessImageName; the current directory is read from the process
// parameters of its PEB. A process that cannot be opened (protected, or another user's) is skipped.
func usersOf(ctx context.Context, roots []string) ([]int, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, fmt.Errorf("snapshot the processes: %w", err)
	}
	defer windows.CloseHandle(snapshot) //nolint:errcheck // nothing to recover from a failed close
	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	var pids []int
	for err = windows.Process32First(snapshot, &entry); err == nil; err = windows.Process32Next(snapshot, &entry) {
		if ctx.Err() != nil {
			return pids, fmt.Errorf("scan the processes: %w", ctx.Err())
		}
		if usesRoots(entry.ProcessID, roots) {
			pids = append(pids, int(entry.ProcessID))
		}
	}
	return pids, nil
}

// usesRoots reports whether the executable or the current directory of the process is inside the roots.
func usesRoots(pid uint32, roots []string) bool {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.PROCESS_VM_READ, false, pid)
	if err != nil {
		return false
	}
	defer windows.CloseHandle(handle) //nolint:errcheck // nothing to recover from a failed close
	if exe, exeErr := imagePath(handle); exeErr == nil && withinAny(roots, exe) {
		return true
	}
	cwd, err := currentDirectory(handle)
	return err == nil && withinAny(roots, cwd)
}

func imagePath(handle windows.Handle) (string, error) {
	buffer := make([]uint16, windows.MAX_LONG_PATH)
	size := uint32(len(buffer))
	if err := windows.QueryFullProcessImageName(handle, 0, &buffer[0], &size); err != nil {
		return "", fmt.Errorf("query the image path: %w", err)
	}
	return windows.UTF16ToString(buffer[:size]), nil
}

// currentDirectory reads the current directory out of the process parameters of the process. It only
// works for processes of the same bitness as the server, which is what the shells and dev servers are.
func currentDirectory(handle windows.Handle) (string, error) {
	var info windows.PROCESS_BASIC_INFORMATION
	if err := windows.NtQueryInformationProcess(handle, windows.ProcessBasicInformation, unsafe.Pointer(&info), uint32(unsafe.Sizeof(info)), nil); err != nil {
		return "", fmt.Errorf("query the process information: %w", err)
	}
	var peb windows.PEB
	if err := readMemory(handle, uintptr(unsafe.Pointer(info.PebBaseAddress)), unsafe.Pointer(&peb), unsafe.Sizeof(peb)); err != nil {
		return "", err
	}
	var params windows.RTL_USER_PROCESS_PARAMETERS
	if err := readMemory(handle, uintptr(unsafe.Pointer(peb.ProcessParameters)), unsafe.Pointer(&params), unsafe.Sizeof(params)); err != nil {
		return "", err
	}
	dir := params.CurrentDirectory.DosPath
	if dir.Length == 0 || dir.Buffer == nil {
		return "", errors.New("the process has no current directory")
	}
	text := make([]uint16, dir.Length/2)
	if err := readMemory(handle, uintptr(unsafe.Pointer(dir.Buffer)), unsafe.Pointer(&text[0]), uintptr(dir.Length)); err != nil {
		return "", err
	}
	return windows.UTF16ToString(text), nil
}

func readMemory(handle windows.Handle, address uintptr, into unsafe.Pointer, size uintptr) error {
	var read uintptr
	if err := windows.ReadProcessMemory(handle, address, (*byte)(into), size, &read); err != nil {
		return fmt.Errorf("read the process memory: %w", err)
	}
	return nil
}

// signalPID terminates pid. Windows has no polite signal for a process without a window, so force changes nothing.
func signalPID(pid int, _ bool) error {
	handle, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
			return errGone
		}
		return fmt.Errorf("open process %d: %w", pid, err)
	}
	defer windows.CloseHandle(handle) //nolint:errcheck // nothing to recover from a failed close
	if err := windows.TerminateProcess(handle, 1); err != nil {
		return fmt.Errorf("terminate process %d: %w", pid, err)
	}
	return nil
}

// extraSpellings returns the long and the 8.3 short spelling of path, which a process may report instead of it.
func extraSpellings(path string) []string {
	var spellings []string
	for _, convert := range []func(*uint16, *uint16, uint32) (uint32, error){windows.GetLongPathName, windows.GetShortPathName} {
		from, err := windows.UTF16PtrFromString(path)
		if err != nil {
			continue
		}
		buffer := make([]uint16, windows.MAX_LONG_PATH)
		if n, err := convert(from, &buffer[0], uint32(len(buffer))); err == nil && n > 0 && int(n) <= len(buffer) {
			spellings = append(spellings, windows.UTF16ToString(buffer[:n]))
		}
	}
	return spellings
}
