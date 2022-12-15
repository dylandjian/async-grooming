package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/slack-go/slack"
)

const TICKET_DATABASE_PATH = "./database.csv"

var (
	teamSizeApproval    int
	emojiValidationName string
	emojiAdmin          string
	firstInitBot        string
	slackDomain         string
	debug               bool
	token               string
	groomingChannelId   string
)

type Status string

const (
	Done       Status = "done"
	InProgress        = "grooming"
	NotStarted        = "notStarted"
)

type Ticket struct {
	status           Status
	messageID        string
	messageTitle     string
	timestamp        string
	approversSlackID []string
}

type SectionBlock struct {
	tickets []Ticket
	message string
}

type TicketTracking struct {
	doneTickets       SectionBlock
	inProgressTickets SectionBlock
	notStartedTickets SectionBlock
}

func findStatusAndApprovers(message slack.Message) (Status, []string) {
	reactions := message.Reactions
	var emojiPresent bool = false
	var approvers []string

	// Check for admin first
	for _, reaction := range reactions {
		if reaction.Name == emojiAdmin {
			approvers = reaction.Users

			return "done", approvers
		}
	}

	for _, reaction := range reactions {
		if reaction.Name == emojiValidationName {
			approvers = reaction.Users

			if reaction.Count >= teamSizeApproval {
				return "done", approvers
			}

			emojiPresent = true
		}

	}

	if !emojiPresent && message.ReplyCount == 0 {
		return "notStarted", approvers
	}

	return "inProgress", approvers
}

func findTitle(text string) string {
	return strings.Split(text, "\n")[0]
}

func createTicket(message slack.Message) Ticket {
	status, approvers := findStatusAndApprovers(message)
	return Ticket{
		status:           status,
		approversSlackID: approvers,
		messageID:        message.ClientMsgID,
		messageTitle:     findTitle(message.Text),
		timestamp:        message.Timestamp,
	}
}

func addTicketMessage(messages []slack.Block, tickets []Ticket, api slack.Client) []slack.Block {
	if len(tickets) == 0 {
		return append(messages, slack.NewSectionBlock(slack.NewTextBlockObject("plain_text", "No tickets here !", false, false), nil, nil))
	}

	for _, ticket := range tickets {
		slackMessageLink, err := api.GetPermalink(&slack.PermalinkParameters{
			Ts:      ticket.timestamp,
			Channel: groomingChannelId,
		})

		if err != nil {
			fmt.Println("Cannot get slack message link")
		}

		message := slack.NewSectionBlock(
			slack.NewTextBlockObject("plain_text", ticket.messageTitle, false, false),
			nil,
			slack.NewAccessory(&slack.ButtonBlockElement{
				Type:     slack.METButton,
				ActionID: "test",
				Text:     slack.NewTextBlockObject("plain_text", "Message", false, false),
				Value:    "value",
				URL:      slackMessageLink,
			}),
		)
		messages = append(messages, message)

		if len(ticket.approversSlackID) > 0 {
			var avatarMessages []slack.MixedElement

			if ticket.status == Done && len(ticket.approversSlackID) < teamSizeApproval {
				avatarMessages = append(avatarMessages, slack.NewTextBlockObject("plain_text", "🆗", true, false))
			} else {
				avatarMessages = append(avatarMessages, slack.NewTextBlockObject("plain_text", "✅", true, false))
			}

			for _, approverSlackID := range ticket.approversSlackID {
				user, err := api.GetUserInfo(approverSlackID)

				if err != nil {
					fmt.Println("err", err)
				}

				imageBlock := slack.NewImageBlockElement(user.Profile.Image72, "profile")
				avatarMessages = append(avatarMessages, *imageBlock)
			}

			messages = append(messages, slack.NewContextBlock(message.BlockID, avatarMessages...))
		}
	}

	return messages
}

func createMessage(tickets TicketTracking, diffTickets []Ticket, api slack.Client) []slack.Block {
	var finalMessage []slack.Block

	greeting := "Hello :wave:, _Grooming Bot_ at your service! @here\nHere is your daily update on the current tickets :smile:"
	finalMessage = append(finalMessage, slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", greeting, false, false), nil, nil))

	finalMessage = append(finalMessage, slack.NewHeaderBlock(slack.NewTextBlockObject("plain_text", tickets.doneTickets.message, false, false)))
	finalMessage = append(finalMessage, slack.NewDividerBlock())
	finalMessage = addTicketMessage(finalMessage, diffTickets, api)

	finalMessage = append(finalMessage, slack.NewHeaderBlock(slack.NewTextBlockObject("plain_text", tickets.inProgressTickets.message, false, false)))
	finalMessage = append(finalMessage, slack.NewDividerBlock())
	finalMessage = addTicketMessage(finalMessage, tickets.inProgressTickets.tickets, api)

	finalMessage = append(finalMessage, slack.NewHeaderBlock(slack.NewTextBlockObject("plain_text", tickets.notStartedTickets.message, false, false)))
	finalMessage = append(finalMessage, slack.NewDividerBlock())
	finalMessage = addTicketMessage(finalMessage, tickets.notStartedTickets.tickets, api)

	goodBye := "Thanks for your attention, _Grooming Bot_"
	finalMessage = append(finalMessage, slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", goodBye, false, false), nil, nil))

	return finalMessage
}

func parseDatabase() ([]Ticket, time.Time) {
	var oldTickets []Ticket
	var header []string

	csvfile, err := os.Open(TICKET_DATABASE_PATH)
	if err != nil {
		log.Fatalln("Couldn't open the csv file", err)
	}

	r := csv.NewReader(csvfile)
	for {
		// Read each record from csv
		record, err := r.Read()
		if err == io.EOF {
			break
		}

		if header == nil {
			header = record
			continue
		}

		if err != nil {
			log.Fatal(err)
		}

		oldTickets = append(oldTickets, Ticket{
			status:       "done",
			messageID:    record[0],
			messageTitle: record[1],
			timestamp:    record[2],
		})
	}

	if len(oldTickets) == 0 {
		firstTime, err := time.Parse("2006-01-02", firstInitBot)

		if err != nil {
			panic(err)
		}

		return oldTickets, firstTime
	}

	lastDoneTicket, err := strconv.ParseInt(oldTickets[len(oldTickets)-1].timestamp[0:10], 10, 64)
	if err != nil {
		panic(err)
	}

	tm := time.Unix(lastDoneTicket, 0)

	return oldTickets, tm
}

func diff(oldTickets []Ticket, newTickets TicketTracking) []Ticket {
	var newDoneTickets []Ticket

	for _, ticket := range newTickets.doneTickets.tickets {
		var exists bool = false

		for _, oldTicket := range oldTickets {
			if oldTicket.messageID == ticket.messageID {
				exists = true
			}
		}

		if !exists {
			newDoneTickets = append(newDoneTickets, ticket)
		}
	}

	return newDoneTickets
}

func isThread(message slack.Message) bool {
	if message.ThreadTimestamp != "" {
		return true
	}

	return false
}

func isBot(message slack.Message) bool {
	if message.BotID != "" {
		return true
	}

	return false
}

func isJoiningOrLeavingMessage(message slack.Message) bool {
	if strings.Contains(message.Text, "has joined") || strings.Contains(message.Text, "has left") {
		return true
	}

	return false
}

func isWrongFormat(message slack.Message) bool {
	matched, _ := regexp.MatchString(`\[(.*)](.*)`, strings.Split(message.Text, "\n")[0])

	return !matched
}

func createTracking(messages []slack.Message) TicketTracking {
	newTickets := TicketTracking{
		doneTickets: SectionBlock{
			message: "Tickets ready to be groomed",
			tickets: make([]Ticket, 0),
		},
		inProgressTickets: SectionBlock{
			message: "Tickets currently being groomed",
			tickets: make([]Ticket, 0),
		},
		notStartedTickets: SectionBlock{
			message: "Tickets that need update !!",
			tickets: make([]Ticket, 0),
		},
	}

	for _, message := range messages {
		if message.Type != "message" || isThread(message) || isBot(message) || isJoiningOrLeavingMessage(message) || isWrongFormat(message) {
			continue
		}

		currentTicket := createTicket(message)

		switch currentTicket.status {
		case "done":
			newTickets.doneTickets.tickets = append(newTickets.doneTickets.tickets, currentTicket)
		case "inProgress":
			newTickets.inProgressTickets.tickets = append(newTickets.inProgressTickets.tickets, currentTicket)
		case "notStarted":
			newTickets.notStartedTickets.tickets = append(newTickets.notStartedTickets.tickets, currentTicket)
		}
	}

	return newTickets
}

func writeCsv(newDoneTickets []Ticket) {
	f, err := os.OpenFile(TICKET_DATABASE_PATH, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)

	if err != nil {
		panic(err)
	}

	w := csv.NewWriter(f)
	for _, ticket := range newDoneTickets {
		w.Write([]string{ticket.messageID, ticket.messageTitle, ticket.timestamp})
	}

	w.Flush()
}

func debugBlock(blocks []slack.Block) {
	b, err := json.MarshalIndent(blocks, "", "    ")
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(string(b))
	return
}

func verifyFlags() {
	if token == "" {
		panic("The token must be defined")
	}

	if groomingChannelId == "" {
		panic("The groomingChannelId must be defined")
	}

	if slackDomain == "" {
		panic("The slackDomain must be defined")
	}
}

func main() {
	flag.BoolVar(&debug, "debug", false, "Show JSON output")
	flag.StringVar(&token, "token", "", "Slack Token")
	flag.StringVar(&groomingChannelId, "groomingChannelId", "", "SlackID of the grooming channel")
	flag.IntVar(&teamSizeApproval, "teamSizeApproval", 3, "Number of approval required to move a ticket")
	flag.StringVar(&emojiValidationName, "emojiValidationName", "white_check_mark", "Emoji name to verify the number of validations")
	flag.StringVar(&emojiAdmin, "emojiAdmin", "ok", "Emoji to bypass the team size approval")
	flag.StringVar(&firstInitBot, "firstInitBot", "", "First date at which to start fetching messages from")
	flag.StringVar(&slackDomain, "slackDomain", "", "Slack domain in which to query")

	flag.Parse()

	verifyFlags()

	// Old tickets saved in CSV
	oldTickets, lastDate := parseDatabase()

	// Get the last 14 days worth of Slack message in the provided channelID
	api := slack.New(token, slack.OptionDebug(debug))
	historyParams := slack.GetConversationHistoryParameters{
		ChannelID: groomingChannelId,
		Oldest:    strconv.FormatInt(lastDate.AddDate(0, 0, -14).Unix(), 10),
	}
	slackHistory, err := api.GetConversationHistory(&historyParams)

	if err != nil {
		panic(err)
	}

	newTicketsTracking := createTracking(slackHistory.Messages)
	diffTickets := diff(oldTickets, newTicketsTracking)

	if len(diffTickets) > 0 {
		writeCsv(diffTickets)
	}

	slackResponse := createMessage(newTicketsTracking, diffTickets, *api)
	api.PostMessage(groomingChannelId, slack.MsgOptionBlocks(slackResponse...))

	if err != nil {
		fmt.Println(err)
	}
}
