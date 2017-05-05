package main

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

func main() {
	authToken := os.Getenv("ACCESS_TOKEN")
	if authToken == "" {
		log.Fatalf("environment variable ACCESS_TOKEN not set")
	}

	port := os.Getenv("PORT")
	if port == "" {
		log.Fatalf("environment variable PORT not set")
	}

	// Triggering gorouting to update information.
	go func() {
		for {
			updateInfo(authToken)
			time.Sleep(24 * time.Hour)
		}
	}()

	fs := http.FileServer(http.Dir("assets"))
	http.Handle("/", fs)

	log.Printf("Listening port %s\n", port)
	http.ListenAndServe(":"+port, nil)
}

var client = &http.Client{
	Timeout: time.Second * 30,
	Transport: &http.Transport{
		Dial: (&net.Dialer{
			Timeout: 30 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 30 * time.Second,
	},
}

const (
	orgsURL       = "https://api.github.com/orgs/ufcg-lsd/members"
	pushEventName = "PushEvent"
	etagsPath     = "./etags.json"
	commitsPath   = "./assets/commits.csv"
	pushesPath    = "./assets/pushes.csv"
	usersPath     = "./assets/users.csv"
)

type User struct {
	Login     string `json:"login"`
	EventsURL string `json:"events_url"`
}

type Event struct {
	Type      string    `json:"type"`
	CreatedAt time.Time `json:"created_at"`
	Payload   struct {
		Size int `json:"size"`
	} `json:"payload"`
	Actor struct {
		Login string `json:"login"`
	} `json:"actor"`
}

type DailyInfo struct {
	Date    time.Time
	Commits int
	Pushes  int
	Users   int
}

type DailyInfoSlice []DailyInfo

func (p DailyInfoSlice) Len() int {
	return len(p)
}

func (p DailyInfoSlice) Less(i, j int) bool {
	return p[i].Date.Before(p[j].Date)
}

func (p DailyInfoSlice) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

type Etag struct {
	User string `json:"user"`
	Etag string `json:"etag"`
}

func updateInfo(authToken string) {
	// Create request;
	req, err := http.NewRequest("GET", orgsURL, nil)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Add("Authorization", "token "+authToken)

	// Send request to fetch org members.
	resp, err := client.Do(req)
	if err != nil {
		log.Print("error fetching org members:%q\n", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respStr, err := httputil.DumpResponse(resp, true)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("error fetching org members (bad status code). response dump:%s\n", string(respStr))
	}

	// Decoding response.
	var users []User
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&users); err != nil {
		log.Printf("error decoding org members response: %q\n", err)
	}
	log.Printf("Users from org fetched successfully. NumUsers:%d", len(users))

	// Reading etags into a map for improving lookups.
	etags, err := readEtags()
	if err != nil {
		log.Printf("error reading etags file:%q ", err)
	}

	var wg sync.WaitGroup
	events := make(chan Event)

	for _, u := range users {
		eventURL := strings.Replace(u.EventsURL, "{/privacy}", "", 1)
		wg.Add(1)
		go func(url, user string) {
			defer wg.Done()

			req, err := http.NewRequest("GET", url, nil)
			if err != nil {
				log.Printf("error creating request to fetch events. user:%s err:%q\n", user, err)
			}

			// More info at: https://developer.github.com/v3/activity/events/
			etag, ok := etags[user]
			if ok {
				req.Header.Add("If-None-Match", etag)
			}
			req.Header.Add("Authorization", "token "+authToken)

			resp, err := client.Do(req)
			if err != nil {
				log.Printf("error trying to fetch events: URL:%s, err:%q\n", url, err)
				return
			}
			defer resp.Body.Close()

			// This is expected if nothing has happened when last polling.
			// https://developer.github.com/v3/activity/events/
			if resp.StatusCode == http.StatusNotModified {
				log.Printf("no new information about the user:%s\n", user)
				return
			}

			if resp.StatusCode != http.StatusOK {
				respStr, err := httputil.DumpResponse(resp, true)
				if err != nil {
					log.Fatal(err)
				}
				log.Printf("error fetching org members (bad status code). url:%s response dump:%s\n", url, string(respStr))
				return
			}

			// Decoding event and publishing it to channel.
			var userEvents []Event
			dec := json.NewDecoder(resp.Body)
			if err := dec.Decode(&userEvents); err != nil {
				log.Printf("error decoding event body:%q\n", err)
			}
			for _, e := range userEvents {
				events <- e
			}

			// Updating etags map with new etag for the user.
			etags[user] = resp.Header.Get("ETag")
		}(eventURL, u.Login)
	}
	go func() {
		wg.Wait()
		close(events)
		log.Printf("Successfully fetched all events from users.")
	}()

	// Writing results
	info := make(map[time.Time]DailyInfo)
	for e := range events {
		if e.Type == pushEventName {
			// Info
			y, m, d := e.CreatedAt.Date()
			date := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
			i := info[date]
			i.Users++
			i.Commits += e.Payload.Size
			i.Pushes++
			i.Date = date
			info[date] = i
		}
	}
	if err := writeResults(info); err != nil {
		log.Printf("error writing results: %q\n", err)
	}
	log.Printf("Results written successfully.\n")

	// Writing etags back to file (as new users might joined the org).
	if err := writeEtags(etags); err != nil {
		log.Printf("error writing etags: %q\n", err)

	}
	log.Printf("Etags written successfully.\n")
}

func createOrWriteOnly(path string) (*os.File, error) {
	var f *os.File
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		log.Printf("Creating file %s\n", path)
		f, err = os.Create(path)
		if err != nil {
			return nil, err
		}
	} else {
		log.Printf("Opening file %s\n", path)
		f, err = os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			return nil, err
		}
	}
	return f, nil
}

func writeResults(info map[time.Time]DailyInfo) error {
	// Creating CSV containing results.
	usersFile, err := createOrWriteOnly(usersPath)
	if err != nil {
		return err
	}
	defer usersFile.Close()
	usersWriter := csv.NewWriter(bufio.NewWriter(usersFile))
	usersWriter.Flush()

	pushesFile, err := createOrWriteOnly(pushesPath)
	if err != nil {
		return err
	}
	defer pushesFile.Close()
	pushesWriter := csv.NewWriter(bufio.NewWriter(pushesFile))
	defer pushesWriter.Flush()

	commitsFile, err := createOrWriteOnly(commitsPath)
	if err != nil {
		return err
	}
	defer commitsFile.Close()
	commitsWriter := csv.NewWriter(bufio.NewWriter(commitsFile))
	defer commitsWriter.Flush()

	var dailyInfoSlice DailyInfoSlice
	for _, i := range info {
		dailyInfoSlice = append(dailyInfoSlice, i)
	}

	sort.Sort(dailyInfoSlice)
	for _, i := range dailyInfoSlice {
		date := strconv.FormatInt(i.Date.Unix(), 10)
		commitsWriter.Write([]string{date, strconv.Itoa(i.Commits)})
		pushesWriter.Write([]string{date, strconv.Itoa(i.Pushes)})
		usersWriter.Write([]string{date, strconv.Itoa(i.Users)})
	}
	return nil
}

func readEtags() (map[string]string, error) {
	f, err := ioutil.ReadFile(etagsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]string), nil
		}
		return nil, err
	}
	var etags []Etag
	if err := json.Unmarshal(f, &etags); err != nil {
		return nil, err
	}
	etagsMap := make(map[string]string)
	for _, e := range etags {
		etagsMap[e.User] = e.Etag
	}
	return etagsMap, nil
}

func writeEtags(m map[string]string) error {
	var etags []Etag
	for u, e := range m {
		etags = append(etags, Etag{User: u, Etag: e})
	}
	out, err := json.MarshalIndent(etags, "", " ")
	if err != nil {
		return err
	}
	f, err := os.Create(etagsPath)
	defer f.Close()
	writer := bufio.NewWriter(f)
	defer writer.Flush()
	fmt.Fprint(writer, string(out))
	return nil
}
