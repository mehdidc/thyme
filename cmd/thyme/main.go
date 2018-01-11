package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/jessevdk/go-flags"
	_ "github.com/mattn/go-sqlite3"
	"github.com/mehdidc/thyme"
	"log"
	"os"
	"runtime"
)

var CLI = flags.NewNamedParser("thyme", flags.PrintErrors|flags.PassDoubleDash)

func init() {
	CLI.Usage = `
thyme - automatically track which applications you use and for how long.

  \|//   thyme is a simple time tracker that tracks active window names and collects
 W Y/    statistics over active, open, and visible windows. Statistics are collected
  \|  ,  into a local JSON file, which is used to generate a pretty HTML report.
   \_/
    \
     \_  thyme is a local CLI tool and does not send any data over the network.

Example usage:

  thyme dep
  thyme track -o <file>
  thyme show  -i <file> -w stats > viz.html

`

	if _, err := CLI.AddCommand("track", "record current windows", "Record current window metadata as JSON printed to stdout or a file. If a filename is specified and the file already exists, Thyme will append the new snapshot data to the existing data.", &trackCmd); err != nil {
		log.Fatal(err)
	}
	if _, err := CLI.AddCommand("show", "visualize data", "Generate an HTML page visualizing the data from a file written to by `thyme track`.", &showCmd); err != nil {
		log.Fatal(err)
	}
	if _, err := CLI.AddCommand("dep", "dep install instructions", "Show installation instructions for required external dependencies (which vary depending on your OS and windowing system).", &depCmd); err != nil {
		log.Fatal(err)
	}
}

// TrackCmd is the subcommand that tracks application usage.
type TrackCmd struct {
	Out string `long:"out" short:"o" description:"output file"`
}

var trackCmd TrackCmd

func (c *TrackCmd) Execute(args []string) error {
	t, err := getTracker()
	if err != nil {
		panic(err)
	}
	snap, err := t.Snap()
	if err != nil {
		panic(err)
	}

	filename := os.Getenv("HOME") + "/.thyme/thyme.db"
	db, err := sql.Open("sqlite3", filename)
	if err != nil {
		panic(err)
	}
	out, err := json.Marshal(snap)
	if err != nil {
		panic(err)
	}
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS data(time TIMESTAMP PRIMARY KEY, value TEXT)")
	if err != nil {
		panic(err)
	}
	stmt, err := db.Prepare("INSERT INTO data(time, value) values(?,?)")
	if err != nil {
		panic(err)
	}
	_, err = stmt.Exec(snap.Time, out)
	if err != nil {
		panic(err)
	}
	if c.Out != "" {
		var value string
		rows, err := db.Query("SELECT value FROM data")
		if err != nil {
			panic(err)
		}
		f, err := os.Create(c.Out)
		f.WriteString("{\n")
		f.WriteString("\"Snapshots\" : [\n")
		rows.Next()
		err = rows.Scan(&value)
		f.WriteString(value)
		for rows.Next() {
			f.WriteString(",")
			err = rows.Scan(&value)
			f.WriteString(value)
		}
		f.WriteString("]\n")
		f.WriteString("}")
		rows.Close()
	}

	return nil
}

// ShowCmd is the subcommand that reads the data emitted by the track
// subcommand and displays the data to the user.
type ShowCmd struct {
	In   string `long:"in" short:"i" description:"input file"`
	What string `long:"what" short:"w" description:"what to show {list,stats}" default:"list"`
}

var showCmd ShowCmd

func (c *ShowCmd) Execute(args []string) error {
	if c.In == "" {
		var snap thyme.Snapshot
		if err := json.NewDecoder(os.Stdin).Decode(&snap); err != nil {
			return err
		}
		for _, w := range snap.Windows {
			fmt.Printf("%+v\n", w.Info())
		}
	} else {
		var stream thyme.Stream
		f, err := os.Open(c.In)
		if err != nil {
			return err
		}
		defer f.Close()

		if err := json.NewDecoder(f).Decode(&stream); err != nil {
			return err
		}
		switch c.What {
		case "stats":
			if err := thyme.Stats(&stream); err != nil {
				return err
			}
		case "list":
			fallthrough
		default:
			fmt.Println(stream)
			thyme.List(&stream)
		}
	}
	return nil
}

type DepCmd struct{}

var depCmd DepCmd

func (c *DepCmd) Execute(args []string) error {
	t, err := getTracker()
	if err != nil {
		return err
	}
	fmt.Println(t.Deps())
	return nil
}

func main() {
	run := func() error {
		_, err := CLI.Parse()
		if err != nil {
			if _, isFlagsErr := err.(*flags.Error); isFlagsErr {
				CLI.WriteHelp(os.Stderr)
				return nil
			} else {
				return err
			}
		}
		return nil
	}

	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func getTracker() (thyme.Tracker, error) {
	switch runtime.GOOS {
	case "windows":
		return thyme.NewTracker("windows"), nil
	case "darwin":
		return thyme.NewTracker("darwin"), nil
	default:
		return thyme.NewTracker("linux"), nil
	}
}
