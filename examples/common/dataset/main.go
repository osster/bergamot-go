package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
)

var messages = []string{
	"Hello, world.",
	"Good morning.",
	"Thank you for your help.",
	"The train leaves at nine o'clock.",
	"Please close the window.",
	"We are learning a new language.",
	"The weather is sunny today.",
	"She reads a book every evening.",
	"Could you send me the address?",
	"The meeting starts after lunch.",
	"I would like a cup of coffee.",
	"Our office is near the central station.",
	"They moved to a small town last year.",
	"The children are playing in the garden.",
	"Please remember to bring your ticket.",
	"This road leads to the old castle.",
	"We need more time to finish the project.",
	"My sister works at a local hospital.",
	"The museum is closed on Mondays.",
	"He usually walks to work in the morning.",
	"Can we reserve a table for four people?",
	"The package arrived earlier than expected.",
	"I left my keys on the kitchen table.",
	"Their new apartment has a beautiful view.",
	"The bus was crowded, so we took a taxi.",
	"She is preparing dinner for her family.",
	"Please turn off the lights when you leave.",
	"We visited the market and bought fresh apples.",
	"The library has a quiet room for studying.",
	"He called his friend to explain the problem.",
	"Our flight was delayed because of heavy rain.",
	"The restaurant serves breakfast until eleven.",
	"I will check the schedule and call you back.",
	"They found a comfortable place to stay near the beach.",
	"The teacher answered every question patiently.",
	"We should leave early to avoid the traffic.",
	"A new bridge connects the two sides of the river.",
	"She sent a message to confirm the appointment.",
	"The store offers a discount to students this week.",
	"Please keep your receipt in case you need to return the item.",
	"He learned how to repair the bicycle from his neighbor.",
	"The conference brings together researchers from many countries.",
	"We can take the scenic route through the mountains.",
	"Her presentation explained the results in simple terms.",
	"The team will review the proposal before making a decision.",
	"I have been waiting for this delivery since Tuesday.",
	"The city is building more homes close to public transport.",
	"After the storm passed, the sky became clear again.",
	"They plan to visit several historic sites during their holiday.",
	"Please let us know if you have any questions about the instructions.",
}

func main() {
	count := flag.Int("count", 1000, "number of messages to generate")
	pair := flag.String("pair", "en-de", "language pair for each message")
	flag.Parse()

	if *count < 1 || strings.TrimSpace(*pair) == "" || strings.ContainsAny(*pair, "\t\r\n") {
		fmt.Fprintln(os.Stderr, "count must be positive and pair must be a non-empty single field")
		os.Exit(2)
	}

	writer := bufio.NewWriter(os.Stdout)
	for i := 0; i < *count; i++ {
		if _, err := fmt.Fprintf(writer, "%s\t%s\n", *pair, messages[i%len(messages)]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	if err := writer.Flush(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
