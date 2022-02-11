package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/sergi/go-diff/diffmatchpatch"
)

type Status int64

const (
	STATUS_OK Status = iota
	STATUS_ERR
	STATUS_TIMEOUT
)

func (s Status) String() string {
	switch s {
	case STATUS_OK:
		return "OK"
	case STATUS_ERR:
		return "ERROR"
	case STATUS_TIMEOUT:
		return "TIMEOUT"
	}
	return "UNKNOWN STATUS"
}

type Result struct {
	Status Status
	out    string
	err    string
}

type Submission struct {
	Name          string
	CompileResult *Result
	RunResult     *Result
}

func main() {
	// Target folder contains Submissions folder (with raw submissions)
	// and in.txt / out.txt
	targetDir := "project2"
	subDir := filepath.Join(targetDir, "submissions")
	in := filepath.Join(targetDir, "in.txt")
	out := filepath.Join(targetDir, "out.txt")

	// Run Submissions
	subChan := make(chan *Submission)
	go filepath.Walk(subDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		fmt.Printf("Running %s...\n", path)
		// Could in theory spawn goroutine for each submission but that would
		// mean potential cpu max + incorrect runtimes
		runSubmission(path, in, subChan)
		return nil
	})

	// Read Submissions / write reports
	repDir := filepath.Join(targetDir, "reports")
	os.RemoveAll(repDir)
	os.Mkdir(repDir, 0777)

	ff, err := os.ReadDir(subDir)
	if err != nil {
		panic(err)
	}

	outFile, err := os.ReadFile(out)
	if err != nil {
		panic(err)
	}
	expectedOut := string(outFile)

	repChan := make(chan bool)
	for i := 0; i < len(ff); i++ {
		s := <-subChan
		fmt.Printf("Writing report for %s...\n", s.Name)
		go writeReport(repDir, &expectedOut, s, repChan)
	}

	for i := 0; i < len(ff); i++ {
		_ = <-repChan
	}

	fmt.Println("All Reports Completed. Exiting...")
	fmt.Println("Please make sure to check error logs as students may have incongruent filenames to class names!!")
}

func runSubmission(path string, in string, subChan chan *Submission) error {
	dir, className := makeTestDir(path)

	sub := &Submission{
		Name: dir,
	}

	sub.CompileResult = runCompile(dir, className)
	if sub.CompileResult.Status == STATUS_ERR {
		subChan <- sub
		os.RemoveAll(dir)
		return nil
	}

	var err error
	sub.RunResult, err = runExec(dir, className, in, 5)

	os.RemoveAll(dir)

	if err != nil {
		return err
	}

	subChan <- sub

	return nil
}

func runCompile(dir, className string) *Result {
	// Prepare javac command
	outBuff := &bytes.Buffer{}
	errBuff := &bytes.Buffer{}
	compCmd := exec.Command("javac", filepath.Join(dir, className+".java"))
	compCmd.Stdout = bufio.NewWriter(outBuff)
	compCmd.Stderr = bufio.NewWriter(errBuff)

	// Run compile Command
	err := compCmd.Run()

	compRes := &Result{
		out: outBuff.String(),
		err: errBuff.String(),
	}

	if err != nil {
		compRes.Status = STATUS_ERR
	} else {
		compRes.Status = STATUS_OK
	}

	return compRes
}

func runExec(dir, className, in string, timeoutSec int) (*Result, error) {
	// Prepare run command
	inFile, err := os.Open(in)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	defer inFile.Close()

	outBuff := &bytes.Buffer{}
	errBuff := &bytes.Buffer{}
	runCmd := exec.Command("java", "-classpath", dir, className)
	runCmd.Stdin = inFile
	runCmd.Stdout = bufio.NewWriter(outBuff)
	runCmd.Stderr = bufio.NewWriter(errBuff)

	// Run Command
	done := make(chan error)

	runCmd.Start()
	go func() { done <- runCmd.Wait() }()

	// Start a timer
	timeout := time.After(time.Duration(timeoutSec) * time.Second)
	runRes := &Result{}

	select {
	case <-timeout:
		runCmd.Process.Kill()
		runRes.Status = STATUS_TIMEOUT
	case err = <-done:
		break
	}

	// Store Result
	runRes.out = outBuff.String()
	runRes.err = errBuff.String()

	if runRes.Status != STATUS_TIMEOUT {
		if err != nil {
			runRes.Status = STATUS_ERR
		} else {
			runRes.Status = STATUS_OK
		}
	}

	return runRes, nil
}

func makeTestDir(path string) (string, string) {
	// Get class name
	raw := strings.Split(strings.TrimSuffix(filepath.Base(path), ".java"), "_")
	class := strings.Split(strings.Join(raw[3:], ""), "-")[0]

	// Setup test folder
	dir := strings.TrimSuffix(filepath.Base(path), ".java")
	os.Mkdir(dir, 0777)
	copy(path, filepath.Join(dir, class+".java"))

	return dir, class
}

func copy(src, dst string) (int64, error) {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return 0, err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return 0, fmt.Errorf("%s is not a regular file", src)
	}

	source, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	defer destination.Close()
	nBytes, err := io.Copy(destination, source)
	return nBytes, err
}

func writeReport(repDir string, expectedOut *string, sub *Submission, repChan chan bool) {
	f, err := os.Create(filepath.Join(repDir, sub.Name+".txt"))
	if err != nil {
		fmt.Println(err)
		return
	}
	defer f.Close()

	f.WriteString(fmt.Sprintf("REPORT FOR %s\n\n", strings.Split(sub.Name, "_")[0]))

	// Print Compile Results
	f.WriteString(fmt.Sprintf("COMPILE RESULT: %s\n", sub.CompileResult.Status))
	if sub.CompileResult.Status == STATUS_ERR {
		f.WriteString("ERROR LOG:\n\n")
		f.WriteString(sub.CompileResult.err)
	}
	f.WriteString("OUT LOG:\n\n")
	f.WriteString(sub.CompileResult.out)

	if sub.CompileResult.Status == STATUS_ERR {
		repChan <- true
		return
	}
	// Print Run Results
	f.WriteString(fmt.Sprintf("\n\nRUN RESULT: %s\n", sub.RunResult.Status))
	if sub.RunResult.Status == STATUS_ERR {
		f.WriteString("ERROR LOG:\n\n")
		f.WriteString(sub.RunResult.err)
	}
	f.WriteString("DIFF LOG:\n\n")
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(*expectedOut, sub.RunResult.out, true)
	f.WriteString(dmp.DiffPrettyText(diffs))
	f.WriteString("OUT LOG:\n\n")
	f.WriteString(sub.RunResult.out)

	repChan <- true
}
