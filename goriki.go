package main

import (
    "flag"
    "fmt"
    "time"
    "os"
    "sort"
    "strconv"
    "io/ioutil"
    "encoding/json"
    "path/filepath"
    "regexp"
    "strings"
    "errors"
)

func usage() {
    fmt.Println("Usage: goriki.exe [--config {configfile}]")
    fmt.Println("       --folder {folder} --max-size {filesize}")
    fmt.Println("       --delete-action {action} --deleted-folder {folder}")
    fmt.Println("")
    fmt.Println("  --folder {folder} (Required)")
    fmt.Println("    Target folder to watch.")
    fmt.Println("")
    fmt.Println("  --max-size {filesize} (Required)")
    fmt.Println("    If the amount of file size in target folder(--folder)")
    fmt.Println("    exceeds this file size, delete old files one by one")
    fmt.Println("    until the amount becomes lower than this file size.")
    fmt.Println("    you can use human readable notation like '3MB'")
    fmt.Println("")
    fmt.Println("  --delete-action {action} (Optional, Default is 'erase')")
    fmt.Println("    {action} is one of the followings:")
    fmt.Println("      erase: Erase a file without passing through trash")
    fmt.Println("      move: this option requires --deleted-folder.")
    fmt.Println("      trash: Move to trash.")
    fmt.Println("")
    fmt.Println("  --deleted-folder {folder} (Optional)")
    fmt.Println("    This {action} is one of the followings:")
    fmt.Println("      erase: Erase a file without passing through trash")
    fmt.Println("      move: Move to specified folder. (this option requires --deleted-folder)")
    fmt.Println("      trash: Move to trash.")
    fmt.Println("")
    fmt.Println("  --log-file {filepath} (Optional)")
    fmt.Println("    If this option was given,")
    fmt.Println("    goriki writes all log strings to {filepath}.")
    fmt.Println("")
    fmt.Println("  --config {configfile} (Optional)")
    fmt.Println("    Specify required options by config file.")
    fmt.Println("    When --config and required options are specified together,")
    fmt.Println("    required options which were specified by arguments")
    fmt.Println("    override specified values in config file.")
    fmt.Println("")
    fmt.Println("Author")
    fmt.Println("  tyru <tyru.exe@gmail.com>")
    fmt.Println("")
    fmt.Println("License")
    fmt.Println("  NEW BSD")

    os.Exit(1)
}

func usageErrorMsg(errorMsg string) {
    fmt.Fprintln(os.Stderr, errorMsg)
    fmt.Fprintln(os.Stderr, "")
    fmt.Fprintln(os.Stderr, "please specify --help for a long help.")
    os.Exit(1)
    // time.Sleep(2 * time.Second)
    // usage()
}

type Flags struct {
    folder string
    maxSize string
    maxSizeInt uint64
    deleteAction string
    deletedFolder string
}
var logFileName = ""
var logFile = os.Stdout

func parseFlags() Flags {
    var flags Flags
    var configFile string

    // Parse arguments.
    flag.StringVar(&flags.folder, "folder", "", "target folder")
    flag.StringVar(&flags.maxSize, "max-size", "", "max file size")
    flag.StringVar(&flags.deleteAction, "delete-action", "erase", "action to take when deleting a file")
    flag.StringVar(&flags.deletedFolder, "deleted-folder", "", "folder for '--delete-action move'")
    flag.StringVar(&logFileName, "log-file", "", "filename for log file")
    flag.StringVar(&configFile, "config", "", "config file")

    if len(configFile) != 0 {
        parseConfig(configFile, &flags)
    }
    flag.Parse()

    // Check required values.
    if len(flags.folder) == 0 || len(flags.maxSize) == 0 {
        usageErrorMsg("error: missing required options.")
    }

    // Check values.
    if flags.deleteAction != "erase" &&
       flags.deleteAction != "move" &&
       flags.deleteAction != "trash" {
        usageErrorMsg("error: invalid --delete-action value.")
    }
    if flags.deleteAction == "move" && len(flags.deletedFolder) == 0 {
        usageErrorMsg("error: specified '--delete-action move' but not --deleted-folder.")
    }
    maxSizeInt, err := parseHumanReadableSize(flags.maxSize);
    if err != nil {
        fmt.Fprintf(os.Stderr, "error: you specified invalid format --max-size value.")
        os.Exit(10)
    }
    flags.maxSizeInt = maxSizeInt

    return flags
}

func parseConfig(filename string, flags *Flags) {
    jsonString, err := ioutil.ReadFile(filename)
    if err != nil {
        fmt.Fprintln(os.Stderr, "error: " + err.Error())
        return
    }
    err = json.Unmarshal(jsonString, flags)
    if err != nil {
        fmt.Fprintln(os.Stderr, "error: " + err.Error())
        return
    }
}


type FoundFile struct {
    path string
    size uint64
    mtime time.Time
}


// By is the type of a "less" function that defines the ordering of its FoundFile arguments.
type By func(f1, f2 *FoundFile) bool

// Sort is a method on the function type, By, that sorts the argument slice according to the function.
func (by By) Sort(files []FoundFile) {
    ps := &fileSorter{
        files: files,
        by:      by, // The Sort method's receiver is the function (closure) that defines the sort order.
    }
    sort.Sort(ps)
}

// fileSorter joins a By function and a slice of Planets to be sorted.
type fileSorter struct {
    files []FoundFile
    by      func(f1, f2 *FoundFile) bool // Closure used in the Less method.
}

// Len is part of sort.Interface.
func (s *fileSorter) Len() int {
    return len(s.files)
}

// Swap is part of sort.Interface.
func (s *fileSorter) Swap(i, j int) {
    s.files[i], s.files[j] = s.files[j], s.files[i]
}

// Less is part of sort.Interface. It is implemented by calling the "by" closure in the sorter.
func (s *fileSorter) Less(i, j int) bool {
    return s.by(&s.files[i], &s.files[j])
}


func walkFolder(folder string) (uint64, []FoundFile) {
    var filesize uint64
    var fileList []FoundFile
    filepath.Walk(folder, func(path string, info os.FileInfo, err error) error {
        if info.IsDir() { return nil }
        // if !info.IsRegular() { return nil }
        filesize += uint64(info.Size())
        fileList = append(fileList, FoundFile{path, uint64(info.Size()), info.ModTime()})
        return nil
    })
    return filesize, fileList
}

type HumanReadableSize struct {
    regexp *regexp.Regexp
    unitSize uint64
    unitString string
}
var humanReadableSize []HumanReadableSize = []HumanReadableSize{
    HumanReadableSize{regexp.MustCompile(`^(\d+)TB?$`), 1024 * 1024 * 1024 * 1024, " TiB"},
    HumanReadableSize{regexp.MustCompile(`^(\d+)GB?$`), 1024 * 1024 * 1024, " GiB"},
    HumanReadableSize{regexp.MustCompile(`^(\d+)MB?$`), 1024 * 1024, " MiB"},
    HumanReadableSize{regexp.MustCompile(`^(\d+)KB?$`), 1024, " KiB"},
    HumanReadableSize{regexp.MustCompile(`^(\d+)B?$`), 1, " B"},
}

func parseHumanReadableSize(str string) (uint64, error) {
    str = strings.TrimSpace(str)
    for _, hsize := range humanReadableSize {
        if hsize.regexp.MatchString(str) {
            numstr := hsize.regexp.FindStringSubmatch(str)[1]
            size, err := strconv.ParseUint(numstr, 10, 64)
            if err == nil { return size * hsize.unitSize, nil }
        }
    }
    return 0, errors.New("invalid format")
}

func formatHumanReadableSize(num uint64) string {
    for _, hsize := range humanReadableSize {
        if num > hsize.unitSize {
            return fmt.Sprintf("%f%s",
                   float64(num) / float64(hsize.unitSize), hsize.unitString)
        }
    }
    if num == 0 {
        return "0 B"
    }
    panic("Cannot convert integer to human readable size string.")
}

func log(msg string) {
    fmt.Fprintf(logFile, "[INFO] [%s] %s\n", time.Now().Format(time.StampMilli), msg)
}

type deleteActionType func(filepath string) error

func getDeleteFunc(actionType string) deleteActionType {
    return map[string]deleteActionType{
        "erase" : func(filepath string) error {
            return os.Remove(filepath)
        },
        "move" : func(filepath string) error {
            panic("not implemented yet")     // TODO
        },
        "trash" : func(filepath string) error {
            panic("not implemented yet")     // TODO
        },
    }[actionType]
}

func main() {
    flags := parseFlags()

    // Open log file.
    if len(logFileName) != 0 {
        f, err := os.Open(logFileName)
        if err != nil {
            fmt.Fprintf(os.Stderr, "error: Cannot open log file.")
            os.Exit(11)
        }
        logFile = f
        defer f.Close()
    }

    // Scan folder.
    filesize, fileList := walkFolder(flags.folder)
    log(strconv.Itoa(len(fileList)) + " file(s) are found.")
    log("Total File Size: " + formatHumanReadableSize(filesize))

    // Sort result file list by mtime(older --> newer).
    mtime := func(f1, f2 *FoundFile) bool {
        return f1.mtime.Before(f2.mtime)
    }
    By(mtime).Sort(fileList)

    // Do delete the oldest files one by one.
    var deletedFileNum uint64
    var deletedFileSize uint64
    var failedFileNum uint64
    deleteFunc := getDeleteFunc(flags.deleteAction)
    for i := 0; filesize > flags.maxSizeInt; i++ {
        err := deleteFunc(fileList[i].path)
        if err == nil {
            log("Deleted " + fileList[i].path)
            deletedFileNum++
            deletedFileSize += fileList[i].size
        } else {
            fmt.Fprintf(os.Stderr, "warning: Cannot delete '%s'. skipping...:\n%s\n", fileList[i].path, err)
            failedFileNum++
        }
        filesize -= fileList[i].size
    }

    log("---------- Result Report ----------")
    log("Deleted File(s): " + formatHumanReadableSize(deletedFileNum))
    log("Reduced File Size: " + formatHumanReadableSize(deletedFileSize))
    log("File(s) failed to delete: " + strconv.FormatUint(failedFileNum, 10) + " file(s)")
}
