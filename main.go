package main

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
}

type FileArray []File

func (p FileArray) Len() int           { return len(p) }
func (p FileArray) Less(i, j int) bool { return p[i].size < p[j].size }
func (p FileArray) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func walkFolder(folder string) (int64, []File) {
    var filesize int64
    var fileList []File
    filepath.Walk(folder, func(path string, info os.FileInfo, err error) error {
        if info.IsDir() { return nil }
        filesize += info.Size()
        fileList = append(fileList, File{path, info.Size()})
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
    log("Total File Size: " + strconv.Itoa64(filesize))

    sort.Sort(fileList)

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
