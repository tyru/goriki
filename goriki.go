package main

// FIXME: when both mtime are equal, what should goriki does?
// (both when --max-size and --same-file)

import (
    "flag"
    "fmt"
    "time"
    "os"
    "sort"
    "strconv"
    "path/filepath"
    "regexp"
    "strings"
    "errors"
    "io/ioutil"
    "crypto/sha1"
)

func usage() {
    fmt.Println("Usage: goriki.exe OPTIONS")
    fmt.Println("")
    fmt.Println("DESCRIPTION")
    fmt.Println("  Goriki deletes old files one by one by some trigger events,")
    fmt.Println("  like the amount becomes lower than this file size,")
    fmt.Println("  or newer and same data file(s) are found.")
    fmt.Println("  It can allows you round-robin data management,")
    fmt.Println("  which means you doesn't care disk resource.")
    fmt.Println("")
    fmt.Println("OPTIONS")
    fmt.Println("  You must specify at least one folder option")
    fmt.Println("  and one trigger option.")
    fmt.Println("")
    fmt.Println("  Target folder options")
    fmt.Println("")
    fmt.Println("    --folder {folder} (Required)")
    fmt.Println("      Target folder to watch.")
    fmt.Println("")
    fmt.Println("  Trigger options")
    fmt.Println("    You must specify one or more of those trigger options.")
    fmt.Println("")
    fmt.Println("    --max-size {filesize}")
    fmt.Println("      Trigger delete action if the amount of file size")
    fmt.Println("      in target folder(--folder) exceeds this file size")
    fmt.Println("      you can use human readable notation like '3MB'")
    fmt.Println("")
    fmt.Println("    --same-file")
    fmt.Println("      Trigger delete action if goriki found the file(s)")
    fmt.Println("      which is older than the latest file, and")
    fmt.Println("      has exactly same data.")
    fmt.Println("")
    fmt.Println("  Other options")
    fmt.Println("")
    fmt.Println("    --delete-action {action} (Optional, Default is 'erase')")
    fmt.Println("      {action} is one of the followings:")
    fmt.Println("        erase: Erase a file without passing through trash")
    fmt.Println("        move: this option requires --deleted-folder.")
    fmt.Println("        trash: Move to trash.")
    fmt.Println("")
    fmt.Println("    --deleted-folder {folder} (Optional)")
    fmt.Println("      This {action} is one of the followings:")
    fmt.Println("        erase: Erase a file without passing through trash")
    fmt.Println("        move: Move to specified folder. (this option requires --deleted-folder)")
    fmt.Println("        trash: Move to trash.")
    fmt.Println("")
    fmt.Println("    --log-file {filepath} (Optional)")
    fmt.Println("      If this option was given,")
    fmt.Println("      goriki appends all log messages to {filepath}.")
    fmt.Println("")
    fmt.Println("    --ignore {pattern} (Optional)")
    fmt.Println("      If this option was given,")
    fmt.Println("      goriki does not process folders/files")
    fmt.Println("      matched with {filepath}.")
    fmt.Println("      NOTE: file path is always delimitered by slash (/),")
    fmt.Println("      not backslash (\\).")
    fmt.Println("")
    fmt.Println("    --verbose (Optional)")
    fmt.Println("      More verbose log messages.")
    fmt.Println("")
    fmt.Println("    --quiet (Optional)")
    fmt.Println("      More quiet log messages.")
    fmt.Println("      This option cannot suppress Start & End logs messages.")
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
    fmt.Fprintln(os.Stderr, "please specify --help for more detailed help.")
    os.Exit(1)
    // time.Sleep(2 * time.Second)
    // usage()
}


// These options can be specified also in config file.
type Flags struct {
    folder string
    maxSize string
    maxSizeInt uint64
    sameFile bool
    deleteAction string
    deletedFolder string
    ignorePattern string
}

var LOG_FILENAME = ""
var LOG_FILE = os.Stdout
var LOG_LEVEL int = 1
var LOG_LEVEL_STR = map[int]string{
    2: "DEBUG",
    1: "INFO",
    0: "WARN",
    -999: "START",
    -998: "END",
}
const START_LOGLEVEL = -999
const END_LOGLEVEL = -998


func parseFlags() Flags {
    var flags Flags
    var showLongHelp bool
    var verbose bool
    var quiet bool

    // Parse arguments.
    flag.StringVar(&flags.folder, "folder", "", "target folder")
    flag.StringVar(&flags.maxSize, "max-size", "", "trigger delete action when it exceeds this file size")
    flag.BoolVar(&flags.sameFile, "same-file", false, "trigger delete action when same and older file is found")
    flag.StringVar(&flags.deleteAction, "delete-action", "erase", "action to take when deleting a file")
    flag.StringVar(&flags.deletedFolder, "deleted-folder", "", "folder for '--delete-action move'")
    flag.StringVar(&LOG_FILENAME, "log-file", "", "filename for log file")
    flag.StringVar(&flags.ignorePattern, "ignore", "", "ignore pattern")
    flag.BoolVar(&verbose, "verbose", false, "verbose log messages")
    flag.BoolVar(&quiet, "quiet", false, "quiet log messages")
    flag.BoolVar(&showLongHelp, "help", false, "show long help")

    flag.Parse()

    if verbose { LOG_LEVEL++ }
    if quiet   { LOG_LEVEL-- }

    if showLongHelp {
        usage()
    }

    // Check required values.
    if len(flags.folder) == 0 || (len(flags.maxSize) == 0 && !flags.sameFile) {
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


func walkFolder(
    folder string,
    ignorePattern string,
    sameFile bool,
    fileCh chan *FoundFile,
    totalFileNum *uint64,
    totalFileSize *uint64) {

    var ignoreRegexp *regexp.Regexp
    if len(ignorePattern) != 0 {
        ignoreRegexp = regexp.MustCompile(ignorePattern)
    }
    filepath.Walk(folder, func(path string, fileinfo os.FileInfo, err error) error {
        // if !fileinfo.IsRegular() { return nil }
        if fileinfo.IsDir() { return nil }
        // TODO: Implement filepath.Walk() which can accept ignoring folders/files
        // Because it costs to search also under ignored folders.
        if ignoreRegexp != nil && ignoreRegexp.MatchString(filepath.ToSlash(path)) {
            info("Skipped " + path)
            return nil
        }
        file := FoundFile{
            path,
            uint64(fileinfo.Size()),
            fileinfo.ModTime(),
        }
        *totalFileNum++
        *totalFileSize += uint64(file.size)
        fileCh <- &file
        return nil
    })
    fileCh <- nil
}

func computeHashString(filename string) (string, error) {
    h := sha1.New()
    // FIXME: Do not read all contents at once!
    contents, err := ioutil.ReadFile(filename)
    if err != nil { return "", err }
    h.Write(contents)
    hash := fmt.Sprintf("%x", h.Sum(nil))
    return hash, nil
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

func debug(msg string) {
    log(msg, 2)
}

func info(msg string) {
    log(msg, 1)
}

func warn(msg string) {
    log(msg, 0)
}

func log(msg string, level int) {
    if LOG_LEVEL >= level {
        fmt.Fprintf(LOG_FILE, "[%s] [%s] %s\n", LOG_LEVEL_STR[level], time.Now().Format(time.StampMilli), msg)
    }
}

type deleteFuncType func(filepath FoundFile) error

func makeDeleteFunc(
    actionType string,
    deletedFileNum *uint64,
    deletedFileSize *uint64,
    failedFileNum *uint64,
) deleteFuncType {

    deleteFunc := map[string]deleteFuncType{
        "erase" : func(file FoundFile) error {
            return os.Remove(file.path)
        },
        "move" : func(file FoundFile) error {
            panic("not implemented yet")     // TODO
        },
        "trash" : func(file FoundFile) error {
            panic("not implemented yet")     // TODO
        },
    }[actionType]

    if deleteFunc == nil {
        panic("deleteFunc == nil")
    }

    return func(file FoundFile) error {
        err := deleteFunc(file)
        if err == nil {
            info(fmt.Sprintf("Deleted '%s' (Size: %s, Trigger: --max-size)",
                    file.path, formatHumanReadableSize(file.size)))
            *deletedFileNum++
            *deletedFileSize += file.size
        } else {
            warn(fmt.Sprintf("Cannot delete '%s'. skipping...:\n%s\n", file.path, err))
            *failedFileNum++
        }
        return err
    }
}

func deleteByMaxSize(
    maxSize uint64,
    currentSize uint64,
    fileCh chan *FoundFile,
    deleteFunc deleteFuncType) {

    var fileList []FoundFile

    // Get all file list at the end of walkFolder().
    for {
        file := <-fileCh
        if file == nil { break }
        fileList = append(fileList, *file)
    }

    // Sort result file list by mtime(older --> newer).
    mtime := func(f1, f2 *FoundFile) bool {
        return f1.mtime.Before(f2.mtime)
    }
    By(mtime).Sort(fileList)

    // Do delete the oldest files one by one.
    for _, file := range fileList {
        if currentSize <= maxSize {
            break
        }
        err := deleteFunc(file)
        if err == nil {
            currentSize -= file.size
        }
    }

    fileCh <- nil
}

func deleteBySameFile(
    currentSize uint64,
    fileCh chan *FoundFile,
    deleteFunc deleteFuncType) {

    var sameFile map[string]FoundFile

    for {
        file := <-fileCh
        if file == nil { break }

        hash, err := computeHashString(file.path)
        if err != nil {
            warn(fmt.Sprintf(
                "Can't compute hash of file '%s'. skipping...:\n%s\n",
                file.path, err))
            continue
        }
        if _, keyExists := sameFile[hash]; keyExists {
            if sameFile[hash].mtime.Before(file.mtime) {
                deleteFunc(sameFile[hash])
                sameFile[hash] = *file
            } else {
                // FIXME: when both mtime are equal, what should goriki does?
                deleteFunc(*file)
            }
        } else {
            sameFile[hash] = *file
        }
    }

    // Send remaining files to the next trigger function.
    for _, file := range sameFile {
        fileCh <- &file
    }
    fileCh <- nil
}

func main() {
    flags := parseFlags()

    // Open log file.
    if len(LOG_FILENAME) != 0 {
        f, err := os.OpenFile(LOG_FILENAME, os.O_RDWR|os.O_APPEND, 0660)
        if err != nil {
            fmt.Fprintf(os.Stderr, "error: Cannot open log file.")
            os.Exit(11)
        }
        LOG_FILE = f
        defer f.Close()
    }

    log("---------- Starting goriki ----------", START_LOGLEVEL)

    // TODO: Implement logf()
    debug(fmt.Sprintf("--folder=%s", flags.folder))
    debug(fmt.Sprintf("--max-size=%s", flags.maxSize))
    debug(fmt.Sprintf("--delete-action=%s", flags.deleteAction))
    debug(fmt.Sprintf("--deleted-folder=%s", flags.deletedFolder))
    debug(fmt.Sprintf("--ignore=%s", flags.ignorePattern))

    var totalFileNum uint64
    var totalFileSize uint64

    // Scan folder.
    fileCh := make(chan *FoundFile)
    go walkFolder(flags.folder, flags.ignorePattern, flags.sameFile, fileCh, &totalFileNum, &totalFileSize)

    // Set up deleteFunc().
    var deletedFileNum uint64
    var deletedFileSize uint64
    var failedFileNum uint64

    deleteFunc := makeDeleteFunc(flags.deleteAction, &deletedFileNum, &deletedFileSize, &failedFileNum)

    if flags.sameFile {
        go deleteBySameFile(totalFileSize, fileCh, deleteFunc)
    }
    if len(flags.maxSize) != 0 {
        // --max-size must be syncronous
        // because the oldest file can be determined after sort.
        go deleteByMaxSize(flags.maxSizeInt, totalFileSize, fileCh, deleteFunc)
    }

    // Ignore all remaining files which were not deleted.
    for <-fileCh != nil {}

    info("---------- Result Report ----------")
    info(fmt.Sprintf("Total File(s): %d file(s) (%s)",
            strconv.FormatUint(totalFileNum, 10),
            formatHumanReadableSize(totalFileSize)))
    info(fmt.Sprintf("Deleted File(s): %s file(s) (%s)",
            strconv.FormatUint(deletedFileNum, 10),
            formatHumanReadableSize(deletedFileSize)))
    info(fmt.Sprintf("Current File(s): %s file(s) (%s)",
            strconv.FormatUint(totalFileNum - deletedFileNum, 10),
            formatHumanReadableSize(totalFileSize - deletedFileSize)))
    // info(fmt.Sprintf("Statistics: %d file(s) (%s) ---(deleted %d file(s) (%s))---> %d file(s) (%s)",
    //         totalFileNum,
    //         formatHumanReadableSize(totalFileSize),
    //         deletedFileNum,
    //         formatHumanReadableSize(deletedFileSize),
    //         (totalFileNum - deletedFileNum),
    //         formatHumanReadableSize(totalFileSize - deletedFileSize)))
    info(fmt.Sprintf("File(s) failed to delete: %s file(s)",
            strconv.FormatUint(failedFileNum, 10)))

    log("---------- End goriki ----------", END_LOGLEVEL)
}
