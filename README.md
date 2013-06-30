
# Lazy Tool for Round-Robin Life (case 1: Me)

`Dropbox/pdf` folder is copied to NAS folder
so there is no matter to delete `Dropbox/pdf` folder.

    Dropbox/pdf --> NAS

and I want to delete old(\*1) files not to concern disk (and Dropbox) resource.
(like Mac's TimeMachine backup management. I like the way for data ops)

    Dropbox/pdf (Maximum size = 2GB)

`goriki` is the tool for it.
It deletes old files until total file size becomes lower than specified file size.
You can create a task to invoke `goriki` from Windows TaskScheduler like the following:

    C:\path\to\goriki.exe --folder C:\Users\user\Dropbox\pdf --max-size 2G --verbose --log-file C:\Users\user\Dropbox\goriki.log --ignore .organizer (\*2)


See `goriki.exe --help` for more detailed help of each option.


\*1: `old` is determined by modification time.
\*2: `--ignore .organizer`: for ignoring `.organizer` folder created by scanner [ScanSnap iX500](http://scansnap.fujitsu.com/jp/product/ix500/) (Japanese).


# Environment

I use `goriki` only on Windows.
but this might work on Unix like environment
because this is very simple tool.


# Another Round-Robin Tool

Sometimes I forget erase all files in Trash of Desktop.
[NonRccDel](http://homepage2.nifty.com/nonnon/) (Japanese) can erase old files in Trash
which are older than specified day(s) by command-line.
I created a task for also NonRccDel.
