package goriki

import (
    "flag"
    "fmt"
    "time"
    "os"
    "sort"
    "strconv"
    "path/filepath"
)

func usage() {
    fmt.Println("Usage: goriki.exe [--config {configfile}]")
    fmt.Println("       --folder {folder} --size {filesize}")
    fmt.Println("       --delete-action {action} --deleted-folder {folder}")
    fmt.Println("")
    fmt.Println("  --folder {folder} (Required)")
    fmt.Println("    Target folder to watch.")
    fmt.Println("")
    fmt.Println("  --size {filesize} (Required)")
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
    time.Sleep(2 * time.Second)
    usage()
}

type Flags struct {
    folder string
    size int64
    deleteAction string
    deletedFolder string
    configFile string
}

func parseFlags() Flags {
    var flags Flags

    // Parse arguments. (TODO: Use struct)
    flag.StringVar(&flags.folder, "folder", "", "target folder")
    flag.Int64Var(&flags.size, "size", -1, "max file size")
    flag.StringVar(&flags.deleteAction, "delete-action", "erase", "action to take when deleting a file")
    flag.StringVar(&flags.deletedFolder, "deleted-folder", "", "folder for '--delete-action move'")
    // flag.StringVar(&flags.configFile, "config", "", "config file")

    // TODO
    // if len(flags.configFile) != 0 {
    //     loadConfig(flags.configFile)
    // }
    flag.Parse()

    // Check required values.
    if len(flags.folder) == 0 || flags.size < 0 {
        usageErrorMsg("error: missing required options.")
    }
    if flags.deleteAction != "erase" &&
       flags.deleteAction != "move" &&
       flags.deleteAction != "trash" {
        usageErrorMsg("error: invalid --delete-action value.")
    }
    if flags.deleteAction == "move" && len(flags.deletedFolder) == 0 {
        usageErrorMsg("error: specified '--delete-action move' but not --deleted-folder.")
    }

    return flags
}

type File struct {
    path string
    size int64
    mtime time.Time
}


// By is the type of a "less" function that defines the ordering of its File arguments.
type By func(f1, f2 *File) bool

// Sort is a method on the function type, By, that sorts the argument slice according to the function.
func (by By) Sort(files []File) {
    ps := &fileSorter{
        files: files,
        by:      by, // The Sort method's receiver is the function (closure) that defines the sort order.
    }
    sort.Sort(ps)
}

// fileSorter joins a By function and a slice of Planets to be sorted.
type fileSorter struct {
    files []File
    by      func(f1, f2 *File) bool // Closure used in the Less method.
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


func walkFolder(folder string) (int64, []File) {
    var filesize int64
    var fileList []File
    filepath.Walk(folder, func(path string, info os.FileInfo, err error) error {
        if info.IsDir() { return nil }
        // if !info.IsRegular() { return nil }
        filesize += info.Size()
        fileList = append(fileList, File{path, info.Size(), info.ModTime()})
        return nil
    })
    return filesize, fileList
}

func log(msg string) {
    fmt.Printf("[INFO] [%s] %s\n", time.Now().Format(time.StampMilli), msg)
}

func main() {
    flags := parseFlags()

    filesize, fileList := walkFolder(flags.folder)
    log(strconv.Itoa(len(fileList)) + " file(s) are found.")
    log("Total File Size: " + strconv.FormatInt(filesize, 10))

    mtime := func(f1, f2 *File) bool {
        return f1.mtime.Before(f2.mtime)
    }
    By(mtime).Sort(fileList)

    for i:=0; filesize > flags.size; i++ {
        err := os.Remove(fileList[i].path)
        if err != nil {
            fmt.Fprintf(os.Stderr, "warning: Cannot delete '%s'. skipping...:\n%s\n", fileList[i].path, err)
        } else {
            log("Deleted " + fileList[i].path)
        }
        filesize -= fileList[i].size
    }
}
