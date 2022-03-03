package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
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

type Submission struct {
	Name          string
	CompileResult *Result
	RunResults    []*Result
}

type Result struct {
	Status Status
	out    string
	err    string
}

func main() {
	// Target folder contains Submissions folder (with raw submissions)
	// and testcases folder (with <whatever>.in / .out (MUST BE ORDERED BY NUMBER))
	targetDir := "p3"
	subDir := filepath.Join(targetDir, "submissions")
	testsDir := filepath.Join(targetDir, "testcases")
	timeoutSecs := 5

	in, out := getTestNames(testsDir)

	// Run Submissions
	submissions := make([]*Submission, 0)
	filepath.Walk(subDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		fmt.Printf("Running %s...\n", path)
		sub, err := runSubmission(path, in, timeoutSecs)
		if err != nil {
			return err
		}

		submissions = append(submissions, sub)
		return nil
	})

	// Read Submissions / write reports
	repDir := filepath.Join(targetDir, "reports")
	os.RemoveAll(repDir)
	os.Mkdir(repDir, 0777)

	finishedChan := make(chan bool)
	for _, sub := range submissions {
		fmt.Printf("Writing report for %s...\n", sub.Name)
		go writeReport(repDir, out, sub, finishedChan)
	}

	for i := 0; i < len(submissions); i++ {
		<-finishedChan
	}

	fmt.Println("All Reports Completed. Exiting...")
	fmt.Println("Please make sure to check error logs as students may have incongruent filenames to class names!!")
}

func getTestNames(testsDir string) (in []string, out []string) {
	// Sort in/out files
	in = make([]string, 0)
	out = make([]string, 0)
	filepath.Walk(testsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		testType := strings.Split(path, ".")[1]
		if testType == "in" {
			in = append(in, path)

		} else {
			out = append(out, path)
		}
		return nil
	})
	sort.Strings(in)
	sort.Strings(out)

	return
}

func runSubmission(path string, inFiles []string, timeout int) (*Submission, error) {
	dir, className := makeTestDir(path)

	sub := &Submission{
		Name:       dir,
		RunResults: make([]*Result, 0),
	}

	// Compile
	sub.CompileResult = runCompile(dir, className)
	if sub.CompileResult.Status == STATUS_ERR {
		os.RemoveAll(dir)
		return sub, nil
	}

	// Run test cases
	for _, inFile := range inFiles {
		fmt.Printf("case %s...\n", inFile)
		res, err := runExec(dir, className, inFile, timeout)
		if err != nil {
			return nil, err
		}

		sub.RunResults = append(sub.RunResults, res)
	}
	err := os.RemoveAll(dir)
	if err != nil {
		return nil, err
	}

	return sub, nil
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

func writeReport(repDir string, outs []string, sub *Submission, finishedChan chan bool) error {
	numErr := 0
	numTimeout := 0
	numOk := 0

	for _, res := range sub.RunResults {
		switch res.Status {
		case STATUS_ERR:
			numErr++
		case STATUS_TIMEOUT:
			numTimeout++
		case STATUS_OK:
			numOk++
		}
	}

	f, err := os.Create(filepath.Join(repDir, sub.Name+".txt"))
	if err != nil {
		finishedChan <- false
		return err
	}
	defer f.Close()

	// Print Compile Result
	f.WriteString(fmt.Sprintf("Report For %s\n\n", strings.Split(sub.Name, "_")[0]))
	f.WriteString(fmt.Sprintf("------------------Compile Result: %s------------------\n", sub.CompileResult.Status))
	if sub.CompileResult.Status == STATUS_ERR {
		f.WriteString("Error Log:\n")
		f.WriteString(sub.CompileResult.err + "\n\n")
	}
	if len(sub.CompileResult.out) != 0 {
		f.WriteString("Out Log:\n")
		f.WriteString(sub.CompileResult.out + "\n\n")
	}
	if sub.CompileResult.Status == STATUS_ERR {
		finishedChan <- true
		return nil
	}

	// Print Run Results
	f.WriteString(fmt.Sprintf("------------------Run Results------------------\nTimeout: %d\nError: %d\nNo Timeout/Error: %d\n\n",
		numTimeout, numErr, numOk))

	f.WriteString("Test Cases:\n")
	diffCnt := 0
	for i, res := range sub.RunResults {
		outFile, err := os.ReadFile(outs[i])
		if err != nil {
			finishedChan <- false
			return err
		}
		outText := strings.ReplaceAll(string(outFile), "\r", "")
		f.WriteString(fmt.Sprintf("Case %s: %s\n", outs[i], res.Status))
		if res.Status == STATUS_ERR {
			f.WriteString("Error Log:\n")
			f.WriteString(res.err + "\n\n")
			continue
		}

		dmp := diffmatchpatch.New()
		diffs := dmp.DiffMain(outText, res.out, false)
		diff := dmp.DiffPrettyText(diffs)
		if diff != outText {
			diffCnt++
			f.WriteString("Diff Log:\n\n")
			f.WriteString(diff)
		} else {
			f.WriteString("Diff Log: No Diff!\n\n")
			continue
		}
		f.WriteString("Out Log:\n\n")
		f.WriteString(res.out)
	}

	f.WriteString(fmt.Sprintf("\n\n---------------Number of mismatch test outputs: %d---------------\n\n", diffCnt))

	finishedChan <- false
	return nil
}

func makeTestDir(path string) (dir string, class string) {
	// Get class name
	raw := strings.Split(strings.TrimSuffix(filepath.Base(path), ".java"), "_")
	class = strings.Split(strings.Join(raw[3:], ""), "-")[0]

	// Setup test folder
	dir = strings.TrimSuffix(filepath.Base(path), ".java")
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
