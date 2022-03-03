# Submission Runner

## How To Use

- Add a folder for the project. This folder will include:
    - `submissions`: folder with all RAW java files from canvas submissions (don't need to rename)
    - `testcases`: folder with all testcases. Make sure every test case ends with `.in` or `.out`, and that each `.in` file is alphabetically matched with its `.out` file.
- run `./submissioncheck <target directory> <timeout in seconds>`
- reports put in `<projfolder>/reports`. Be sure to check for compile errors / etc as this program cannot fix all misaligned class / filenames. you can cat the reports in a terminal to get diff highlighting.

## YOU CAN RUN `./submissioncheck help` FOR MORE HELPFUL INFO

## Notes
- This will ignore '-' marks at the end of a program name (usually whenever someone submits multiple times on canvas)
