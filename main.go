package main

import "fmt"
import "log"
import "strings"
import "strconv"
import "sort"
import "time"
import "os"
import "net/http"
import "net/url"
import "github.com/PuerkitoBio/goquery"

func main() {
	checkAvailability(4)
	// checkAvailability(5)
	sendNotification()
}

type Slot struct {
	Time	string
	Price	string
	PriceCh	string
	Seats	int
}

type PushoverClient struct {
	apiKey		string
	userKey		string
	httpClient	*http.Client
}

var availableSlots = make(map[int][]Slot)

func checkAvailability(month int) {
	data := url.Values {
		"bid": 		{"65"},
		"year": 	{"2025"},
		"month":	{strconv.Itoa(month)},
		"adults":	{"2"},
		"children":	{"0"},
		"rnd": 		{"19"},
	}
	res, err := http.PostForm(os.Getenv("VISA_URL"), data)
	if err != nil {
		log.Fatalf("Greece - Request failed: %s", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		log.Fatalf("Greece - Server error: %s", res.Status)
	}

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		log.Fatalf("Parsing failed: %s", err)
	}

	anchorClass := "a.aero_bcal_day_number"
	doc.Find(anchorClass).Each(func(_ int, a *goquery.Selection) {
		td := a.Parent()
		day, err := strconv.Atoi(td.Text())
		if err != nil {
			log.Fatalf("Day parsing failed: %s", err)
		}
		schedulesString, schedulesExist := td.Attr("data-schedule")
		if !schedulesExist {
			fmt.Printf("Schedules not found on day: %d\n", day)
		}
		schedules := strings.Split(schedulesString, "@")
		for _, schedule := range schedules {
			scheduleSplit := strings.Split(schedule, ";")
			if len(scheduleSplit) != 4 {
				continue
			}
			time := scheduleSplit[0]
			price := scheduleSplit[1]
			priceCh := scheduleSplit[2]
			seats, err := strconv.Atoi(scheduleSplit[3])
			if err != nil {
				log.Fatalf("Seat parsing failed: %s", err)
			}
			minSeats := 2
			if seats >= minSeats {
				availableSlots[day] = append(availableSlots[day], Slot {
						Time: time,
						Price: price,
						PriceCh: priceCh,
						Seats: seats,
					})
			}
		}
	})
}

func newPushoverClient(apiKey string, userKey string) *PushoverClient {
	return &PushoverClient{
		apiKey: 	apiKey,
		userKey: 	userKey,
		httpClient:	&http.Client{Timeout: 10 * time.Second},
	}
}

func (pc *PushoverClient) SendNotification(title string, message string) {
	data := url.Values {
		"token": 	{pc.apiKey},
		"user":		{pc.userKey},
		"title":	{title},
		"message":	{message},
		"html":		{"1"},
		"priority": 	{"1"},
	}
	res, err := pc.httpClient.PostForm("https://api.pushover.net/1/messages.json", data)
	if err != nil {
		fmt.Printf("Pushover - Request failed: %s", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		fmt.Printf("Pushover - Server error: %s", res.Status)
	}
}

func sendNotification() {
	sortedDays := make([]int, 0, len(availableSlots))
	for d := range availableSlots {
		sortedDays = append(sortedDays, d)
	}
	sort.Ints(sortedDays)

	pc := newPushoverClient(os.Getenv("PUSHOVER_API_KEY"), os.Getenv("PUSHOVER_USER_KEY"))

	for _, day := range sortedDays {
		var sb strings.Builder
		slots := availableSlots[day]
		for _, slot := range slots {
			sb.WriteString(fmt.Sprintf("\n🕒 %s  •  💶 %s   •  👥 %d seats\n", slot.Time, slot.Price, slot.Seats))
		}
		notificationMessage := sb.String()
		notificationTitle := fmt.Sprintf("📅 Open slot Day %d", day)
		pc.SendNotification(notificationTitle, notificationMessage)
	}
}
