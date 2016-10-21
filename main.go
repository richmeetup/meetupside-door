package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"reflect"
	"sort"
	"strings"
	"time"

	"os"

	"github.com/fatih/color"
)

type MeetupEventCell struct {
	Current    MeetupEvent
	North      *MeetupEventCell
	East       *MeetupEventCell
	South      *MeetupEventCell
	West       *MeetupEventCell
	UpsideDown bool
}

func commandHelpResponse() string {
	return "+=========================================================================+\n" +
		"| The following commands deal with information.                           |\n" +
		"+=========================================================================+\n" +
		"|  COMMAND          |  DESCRIPTION                          |  SHORTHAND  |\n" +
		"|-------------------+---------------------------------------+-------------|\n" +
		"| PLAYERS           | List the players currently in game    | PL          |\n" +
		"| LOOK              | Examine your current surroundings     | L           |\n" +
		"| LOOK <WHO>        | Look at this denizen                  | L <W>       |\n" +
		"| LOOK <DIR>        | Look in this direction                | L <D>       |\n" +
		"| EXITS             | Display available exits               | EX          |\n" +
		"| INVENTORY         | List items in your inventory          | I           |\n" +
		"| STATUS            | Display your status                   | ST          |\n" +
		"| HEALTH            | Display brief status                  | HE          |\n" +
		"| EXPERIENCE        | Display experience                    | EP          |\n" +
		"| SPELLS            | List spells in your spellbook         | SP          |\n" +
		"| HELP              | Display general help message          | ?           |\n" +
		"+=========================================================================+\n"
}

func defaultResponse(event MeetupEvent) string {
	locationColor := color.New(color.FgYellow).SprintfFunc()
	peopleColor := color.New(color.FgMagenta).SprintfFunc()
	itemsColor := color.New(color.FgHiCyan).SprintfFunc()

	// hack to turn colors off while github.com is down
	/*
		locationColor := fmt.Sprintf
		peopleColor := fmt.Sprintf
		itemsColor := fmt.Sprintf
	*/

	response := ""

	peopleHere := ""
	for i, rsvp := range event.RSVPs {
		if i > 0 {
			if i == event.RSVPCount-1 {
				peopleHere += " and "
			} else {
				peopleHere += ", "
			}
		}
		peopleHere += strings.TrimSpace(rsvp.Member.Name)
	}
	if event.RSVPCount > len(event.RSVPs) {
		peopleHere += fmt.Sprintf(" and %d others", event.RSVPCount)
	}

	response += locationColor("You're at %s.\n", strings.TrimSpace(event.Venue.Name))
	response += peopleColor(peopleHere + " are here.\n")

	nowDate := time.Now()
	startDate := time.Unix(event.StartTime/1000.0, 0)
	startingString := ""
	if startDate.Before(nowDate) {
		startingString = "This Meetup started at " + startDate.Format("3:04pm") + "."
	} else if startDate.YearDay() == nowDate.YearDay() && startDate.Year() == nowDate.Year() {
		startingString = "This Meetup is starting at " + startDate.Format("3:04pm") + "."
	} else {
		startingString = "This Meetup is happening " + startDate.Format("Monday, January 02, 2006 at 3:04pm") + "."
	}
	response += itemsColor(startingString + "\n")

	return response
}

func upsideDownResponse(event MeetupEvent) string {
	upsideDownColor := color.New(color.FgCyan).SprintfFunc()

	response := ""

	members := getMeetupMembers(event.Group.Urlname)
	sort.Sort(ByVisited(members))

	peopleHere := ""
	for i, member := range members[:10] {
		if i > 0 {
			if i == len(members)-1 {
				peopleHere += " and "
			} else {
				peopleHere += ", "
			}
		}
		peopleHere += strings.TrimSpace(member.Name)
	}
	if event.RSVPCount > len(event.RSVPs) {
		peopleHere += fmt.Sprintf(" and %d others", event.RSVPCount)
	}

	response += upsideDownColor("You're in the upside down %s.\n", strings.TrimSpace(event.Venue.Name))
	response += upsideDownColor(peopleHere + " are here and very inactive.\n")

	//startDate := time.Unix(event.StartTime/1000.0, 0)

	return response
}

func writeString(conn net.Conn, str string) {
	conn.Write([]byte(str))
}

func getStartingRoom(events []MeetupEvent) MeetupEventCell {
	startingCell := MeetupEventCell{}
	startingCell.Current = events[0]

	currCell := &startingCell
	for _, event := range events[1:] {
		newCell := &MeetupEventCell{}
		newCell.Current = event

		// decide which direction to go and create a room
		availableDirections := []reflect.Value{}
		toAvailableDirections := []reflect.Value{}
		directions := []reflect.Value{
			reflect.ValueOf(currCell).Elem().FieldByName("North"),
			reflect.ValueOf(currCell).Elem().FieldByName("East"),
			reflect.ValueOf(currCell).Elem().FieldByName("South"),
			reflect.ValueOf(currCell).Elem().FieldByName("West"),
		}
		toDirections := []reflect.Value{
			reflect.ValueOf(newCell).Elem().FieldByName("South"),
			reflect.ValueOf(newCell).Elem().FieldByName("West"),
			reflect.ValueOf(newCell).Elem().FieldByName("North"),
			reflect.ValueOf(newCell).Elem().FieldByName("East"),
		}

		for i, direction := range directions {
			if direction.IsNil() {
				availableDirections = append(availableDirections, direction)
				toAvailableDirections = append(toAvailableDirections, toDirections[i])
			}
		}

		if len(availableDirections) > 0 {
			i := rand.Intn(len(availableDirections))
			availableDirections[i].Set(reflect.ValueOf(newCell))
			toAvailableDirections[i].Set(reflect.ValueOf(currCell))
		}

		// decide if we move on or not (25% chance)
		if rand.Intn(100) < 75 || len(availableDirections) <= 1 {
			currCell = newCell
		}
	}

	return startingCell
}

func handleConnection(conn net.Conn, events []MeetupEvent) {
	boldColor := color.New(color.FgWhite, color.Bold).SprintfFunc()
	boldCyanColor := color.New(color.FgCyan, color.Bold).SprintfFunc()
	errorColor := color.New(color.FgRed).SprintfFunc()
	//boldColor := fmt.Sprintf

	currentRoom := getStartingRoom(events)

	writeString(conn, boldColor("Entering Meetupside...\n"))
	writeString(conn, defaultResponse(currentRoom.Current))

	for {
		if message, err := bufio.NewReader(conn).ReadString('\n'); err != nil {
			panic(err)
		} else {
			var response string
			switch strings.TrimSpace(strings.ToUpper(message)) {
			case "HELP", "?": // XXX - fix the HELP command to actually return this stuff
				response = commandHelpResponse()
			case "WHAT":
				memberName := strings.TrimSpace(currentRoom.Current.RSVPs[rand.Intn(len(currentRoom.Current.RSVPs))].Member.Name)
				response = memberName + " says: \"This is " + currentRoom.Current.Name + ". Care to join?\"\n"
			case "LOOK", "L":
				availableDirections := []string{}
				if currentRoom.North != nil {
					availableDirections = append(availableDirections, "north")
				}
				if currentRoom.East != nil {
					availableDirections = append(availableDirections, "east")
				}
				if currentRoom.South != nil {
					availableDirections = append(availableDirections, "south")
				}
				if currentRoom.West != nil {
					availableDirections = append(availableDirections, "west")
				}

				response = boldColor("On a posted placard, you see the following text written in barely legible handwriting:\n") +
					currentRoom.Current.Description + "\n"

				if len(availableDirections) > 0 {
					response += boldColor("\nYou can go in the following directions: "+strings.Join(availableDirections, ", ")) + "\n"
				} else {
					response += errorColor("\nThere are no visible exits.\n")
				}
			case "EAST", "E":
				if currentRoom.East != nil {
					currentRoom = *currentRoom.East
					response = defaultResponse(currentRoom.Current)
				} else {
					response = errorColor("You can't go that way.\n")
				}
			case "WEST", "W":
				if currentRoom.West != nil {
					currentRoom = *currentRoom.West
					response = defaultResponse(currentRoom.Current)
				} else {
					response = errorColor("You can't go that way.\n")
				}
			case "SOUTH", "S":
				if currentRoom.South != nil {
					currentRoom = *currentRoom.South
					response = defaultResponse(currentRoom.Current)
				} else {
					response = errorColor("You can't go that way.\n")
				}
			case "NORTH", "N":
				if currentRoom.North != nil {
					currentRoom = *currentRoom.North
					response = defaultResponse(currentRoom.Current)
				} else {
					response = errorColor("You can't go that way.\n")
				}
			case "DOWN", "D":
				if currentRoom.UpsideDown {
					response = errorColor("You can't go that way.\n")
				} else {
					currentRoom.UpsideDown = true
					response = boldCyanColor("Entering the upside down...\n")
					response = upsideDownResponse(currentRoom.Current)
				}
			case "UP", "U":
				if currentRoom.UpsideDown {
					currentRoom.UpsideDown = false
					response = boldCyanColor("Returning to the real world...\n")
					response = defaultResponse(currentRoom.Current)
				} else {
					response = errorColor("You can't go that way.\n")
				}
			case "SAVE":
				if currentRoom.UpsideDown {
					currentRoom.UpsideDown = false
					response = "Bringing them back to the real world!\n" +
						defaultResponse(currentRoom.Current)
				} else {
					response = errorColor("People are already meeting up! There's no one here to save.\n")
				}
			default:
				if currentRoom.UpsideDown {
					response = upsideDownResponse(currentRoom.Current)
				} else {
					response = defaultResponse(currentRoom.Current)
				}
			}
			writeString(conn, response)
		}
	}
}

type MeetupMember struct {
	ID           int           `json:"id"`
	Name         string        `json:"name"`
	GroupProfile MeetupProfile `json:"group_profile"`
}

type MeetupRSVP struct {
	ID      int          `json:"id"`
	Created int64        `json:"created"`
	Updated int64        `json:"updated"`
	Member  MeetupMember `json:"member"`
}

type MeetupVenue struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	City    string `json:"city"`
	State   string `json:"state"`
	Country string `json:"country"`
	Zip     string `json:"zip"`
}

type MeetupEvent struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"plain_text_description"`
	RSVPs       []MeetupRSVP `json:"rsvp_sample"`
	RSVPCount   int          `json:"yes_rsvp_count"`
	StartTime   int64        `json:"time"`
	Group       MeetupGroup  `json:"group"`
	Venue       MeetupVenue  `json:"venue"`
}

type MeetupGroup struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Urlname string `json:"urlname"`
}

func getMeetupEvents() []MeetupEvent {
	var events []MeetupEvent

	fields := []string{"plain_text_description", "id", "name", "rsvp_sample", "time", "venue", "yes_rsvp_count", "group"}
	url := fmt.Sprintf(
		"https://api.meetup.com/self/calendar?key=%s&fields=%s&only=%s&page=%d",
		os.Getenv("MEETUP_API_KEY"),
		url.QueryEscape(strings.Join(fields, ",")),
		url.QueryEscape(strings.Join(fields, ",")),
		20)

	if request, err := http.NewRequest("GET", url, nil); err != nil {
		panic(err)
	} else {
		client := &http.Client{}
		if response, err := client.Do(request); err != nil {
			panic(err)
		} else {
			if response.StatusCode != http.StatusOK {
				fmt.Printf("Got invalid HTTP response code: status = %d\n", response.StatusCode)
				return []MeetupEvent{}
			}

			defer response.Body.Close()
			if err := json.NewDecoder(response.Body).Decode(&events); err != nil {
				panic(err)
			}
		}
	}

	return events
}

type MeetupProfile struct {
	Visited int64 `json:"visited"`
}

type ByVisited []MeetupMember

func (a ByVisited) Len() int           { return len(a) }
func (a ByVisited) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByVisited) Less(i, j int) bool { return a[i].GroupProfile.Visited < a[j].GroupProfile.Visited }

func getMeetupMembers(urlname string) []MeetupMember {
	var members []MeetupMember

	fields := []string{"group_profile", "id", "name"}
	url := fmt.Sprintf(
		"https://api.meetup.com/%s/members?key=%s&fields=%s&only=%s&page=%d",
		urlname,
		os.Getenv("MEETUP_API_KEY"),
		url.QueryEscape(strings.Join(fields, ",")),
		url.QueryEscape(strings.Join(fields, ",")),
		100)

	if request, err := http.NewRequest("GET", url, nil); err != nil {
		panic(err)
	} else {
		client := &http.Client{}
		if response, err := client.Do(request); err != nil {
			panic(err)
		} else {
			if response.StatusCode != http.StatusOK {
				fmt.Printf("Got invalid HTTP response code: status = %d\n", response.StatusCode)
				return []MeetupMember{}
			}

			defer response.Body.Close()
			if err := json.NewDecoder(response.Body).Decode(&members); err != nil {
				panic(err)
			}
		}
	}

	return members
}

func main() {
	fmt.Println("Starting server...")
	rand.Seed(time.Now().UTC().UnixNano())
	events := getMeetupEvents()

	if len(events) == 0 {
		fmt.Printf("Couldn't find any events for member associated with MEETUP_API_KEY.\n")
		return
	}

	if ln, err := net.Listen("tcp", ":2002"); err != nil {
		panic(err)
	} else {
		for {
			if conn, err := ln.Accept(); err != nil {
				panic(err)
			} else {
				go handleConnection(conn, events)
			}
		}
	}
}
