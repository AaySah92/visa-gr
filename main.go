package main

import "fmt"
import "log"
import "strings"
import "strconv"
import "time"
import "os"
import "slices"
import "net/http"
import "net/url"
import "github.com/PuerkitoBio/goquery"

func main() {
	checkAvailability(time.Month(4), 2025)
	checkAvailability(time.Month(5), 2025)
	sendNotification()
}

type Slot struct {
	TimeSlot	string
	Price		string
	PriceCh		string
	Seats		int
}

type PushoverClient struct {
	apiKey		string
	userKey		string
	httpClient	*http.Client
}

var availableSlots = make(map[time.Time][]Slot)

func checkAvailability(month time.Month, year int) {
	data := url.Values {
		"bid": 		{"65"},
		"year": 	{"2025"},
		"month":	{strconv.Itoa(int(month))},
		"adults":	{"2"},
		"children":	{"0"},
		"rnd": 		{"19"},
	}
	res, err := http.PostForm(os.Getenv("VISA_URL"), data)
	if err != nil {
		log.Println("Greece - Request failed:", err)
		return
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		log.Println("Greece - Server error:", res.Status)
		return
	}

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		log.Println("Parsing failed:", err)
		return
	}

	anchorClass := "a.aero_bcal_day_number"
	doc.Find(anchorClass).Each(func(_ int, a *goquery.Selection) {
		td := a.Parent()
		day, err := strconv.Atoi(td.Text())
		if err != nil {
			log.Println("Day parsing failed:", err)
			return
		}
		schedulesString, schedulesExist := td.Attr("data-schedule")
		if !schedulesExist {
			log.Println("Schedules not found on day:", day)
			return
		}
		schedules := strings.Split(schedulesString, "@")
		for _, schedule := range schedules {
			scheduleSplit := strings.Split(schedule, ";")
			if len(scheduleSplit) != 4 {
				continue
			}
			timeSlot := scheduleSplit[0]
			price := scheduleSplit[1]
			priceCh := scheduleSplit[2]
			seats, err := strconv.Atoi(scheduleSplit[3])
			if err != nil {
				log.Println("Seat parsing failed:", err)
				continue
			}
			minSeats := 2
			if seats >= minSeats {
				date := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
				availableSlots[date] = append(availableSlots[date], Slot {
						TimeSlot: timeSlot,
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
		log.Println("Pushover - Request failed:", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		log.Println("Pushover - Server error:", res.Status)
	}
}

func sendNotification() {
	sortedDates := make([]time.Time, 0, len(availableSlots))
	for d := range availableSlots {
		sortedDates = append(sortedDates, d)
	}
	slices.SortFunc(sortedDates, func(a, b time.Time) int {
		if a.Before(b) {
			return -1
		} else if a.After(b) {
			return 1
		}
		return 0
	})

	pc := newPushoverClient(os.Getenv("PUSHOVER_API_KEY"), os.Getenv("PUSHOVER_USER_KEY"))

	for _, date := range sortedDates {
		var sb strings.Builder
		slots := availableSlots[date]
		for _, slot := range slots {
			sb.WriteString(fmt.Sprintf("\n🕒 %s  •  💶 %s   •  👥 %d seats\n", slot.TimeSlot, slot.Price, slot.Seats))
		}
		notificationMessage := sb.String()
		notificationTitle := fmt.Sprintf("📅 Open Slot on %s", date.Format("Mon 2 Jan"))
		pc.SendNotification(notificationTitle, notificationMessage)
	}
}
