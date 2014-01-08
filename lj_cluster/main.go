// The lj_cluster command line tool performs hierarchical clustering on
// LiveJournal friends lists.
package main

import (
	"flag"
	"fmt"
	"github.com/crowsonkb/cluster"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"runtime"
	"strings"
)

var ljGetUrl = "http://www.livejournal.com/misc/fdata.bml?user="

var inituser string

func ljGet(user string, direction uint8) []string {
	body, err := ioutil.ReadFile(user)
	if err != nil {
		log.Printf("Retrieving data for: %s\n", user)
		resp, err := http.Get(ljGetUrl + user)
		if err != nil {
			log.Fatal(err)
		}
		body, _ = ioutil.ReadAll(resp.Body)
		if err = ioutil.WriteFile(user, body, 0644); err != nil {
			log.Fatal(err)
		}
	}
	result := make([]string, 0)
	lines := strings.Split(string(body), "\n")
	for _, line := range lines {
		if len(line) > 2 && line[0] == direction {
			result = append(result, (line[2:]))
		}
	}
	return result
}

func init() {
	flag.StringVar(&inituser, "user", "",
		"The user whose friends data we will cluster")
	flag.Parse()
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)
	runtime.GOMAXPROCS(runtime.NumCPU())
}

func main() {
	if inituser == "" {
		flag.Usage()
		os.Exit(1)
	}

	if os.Chdir("fdata") != nil {
		if err := os.Mkdir("fdata", 0755); err != nil {
			log.Fatal(err)
		}
		if err := os.Chdir("fdata"); err != nil {
			log.Fatal(err)
		}
	}

	fdata := ljGet(inituser, '>')
	names := make([]string, 0)
	vecs := make([]cluster.Vec, 0)

	for _, user := range fdata {
		if user != inituser {
			names = append(names, user)
			vecs = append(vecs, cluster.NewVec(ljGet(user, '<')))
		}
	}
	log.Println("Clustering...")

	merges := cluster.Cluster(vecs)
	log.Println("Done.")

	clusters := cluster.Interpret(merges)

	for _, cluster := range clusters {
		fmt.Print("[")
		for i, user := range cluster {
			fmt.Print(names[user])
			if i != len(cluster)-1 {
				fmt.Print(" ")
			}
		}
		fmt.Print("]\n\n")
	}
}
