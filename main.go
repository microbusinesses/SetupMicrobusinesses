package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gocql/gocql"
)

var cassandraHosts string
var cassandraProtoclVersion int
var runInParallel bool

func main() {
	flag.StringVar(&cassandraHosts, "cassandra-hosts", "127.0.0.1", "The list of cassandra hosts to connect to. The default value is 127.0.0.1")
	flag.IntVar(&cassandraProtoclVersion, "cassandra-protocl-version", 4, "The cassandra protocl version. The default value is 4.")
	flag.BoolVar(&runInParallel, "run-in-parallel", false, "Indicates whether the scripts should be executed in parallel or one by one. The default value is false.")
	flag.Parse()

	cluster := gocql.NewCluster()
	cluster.Hosts = strings.Split(cassandraHosts, ",")
	cluster.ProtoVersion = cassandraProtoclVersion
	cluster.Consistency = gocql.Quorum
	cluster.Timeout = 10 * time.Second

	session, err := cluster.CreateSession()

	if err != nil {
		log.Fatal(err.Error())

		return
	}

	urls := [5]string{
		"https://raw.githubusercontent.com/micro-business/AddressService/master/DatabaseScript.cql",
		"https://raw.githubusercontent.com/micro-business/TenantService/master/DatabaseScript.cql",
		"https://raw.githubusercontent.com/micro-business/UserService/master/DatabaseScript.cql"}

	if runInParallel {
		errorChannel := make(chan error, len(urls))

		var waitGroup sync.WaitGroup

		for _, url := range urls {
			waitGroup.Add(1)

			go runCqlScriptInParallel(session, errorChannel, &waitGroup, url)
		}

		go func() {
			waitGroup.Wait()
			close(errorChannel)
		}()

		errorMessage := ""
		errorFound := false

		for err := range errorChannel {
			if err != nil {
				errorMessage += err.Error()
				errorMessage += "\n"
				errorFound = true
			}
		}

		if errorFound {
			log.Fatal(errorMessage)
		}
	} else {
		for _, url := range urls {
			err := runCqlScript(session, url)

			if err != nil {
				log.Fatal(err.Error())

				return
			}
		}
	}
}

func runCqlScript(session *gocql.Session, url string) error {
	resp, err := http.Get(url)

	if err != nil {
		return err

	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		return err
	}

	scriptLines := strings.Split(string(body[:len(body)]), "\n")

	for _, scriptLine := range scriptLines {
		scriptLine = strings.TrimSpace(scriptLine)

		if len(scriptLine) != 0 {
			fmt.Println("Running command: " + scriptLine)

			err = session.Query(scriptLine).Exec()

			if err != nil {
				return err
			}
		}
	}

	return nil
}

func runCqlScriptInParallel(session *gocql.Session, errorChannel chan<- error, waitGroup *sync.WaitGroup, url string) {
	defer waitGroup.Done()

	resp, err := http.Get(url)

	if err != nil {
		errorChannel <- err

		return
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		errorChannel <- err

		return
	}

	scriptLines := strings.Split(string(body[:len(body)]), "\n")

	for _, scriptLine := range scriptLines {
		scriptLine = strings.TrimSpace(scriptLine)

		if len(scriptLine) != 0 {
			fmt.Println("Running command: " + scriptLine)

			err = session.Query(scriptLine).Exec()

			if err != nil {
				errorChannel <- err

				return
			}
		}

	}

	errorChannel <- nil
}
