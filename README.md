# Submission Runner

## How To Use

- Make sure you have golang installed
- Add a folder in this root directory for a project. This folder will include:
    - `submissions`: folder with all RAW js files from canvas submissions (don't need to rename)
    - `in.txt` - input test text
    - `out.txt` - expected output text
- run `go run submissions.go`
- reports put in `<projfolder>/reports`. Be sure to check for compile errors / etc as this program cannot fix all misaligned class / filenames

## Notes

- This will ignore '-' marks at the end of a program name (usually whenever someone submits multiple times on canvas)