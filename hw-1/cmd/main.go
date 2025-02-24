package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"gitlab.ozon.dev/pupkingeorgij/homework/handler"
	"gitlab.ozon.dev/pupkingeorgij/homework/internal/storage"
)

func main() {
	stg, err := storage.New("storage.json")
	if err != nil {
		fmt.Println("Storage init error:", err)
		return
	}

	h := handler.New(stg)

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		args := strings.Fields(input)
		if len(args) == 0 {
			continue
		}

		switch args[0] {
		case "exit":
			fmt.Println("Exiting...")
			return
		case "accept":
			h.HandleAccept(args[1:])
		case "return":
			h.HandleReturn(args[1:])
		case "list-orders":
			h.HandleListOrders(args[1:])
		case "help":
			h.HandleHelp()
		case "process":
			h.HandleProcess(args[1:])
		case "list-returns":
			h.HandleListReturns(args[1:])
		case "history":
			h.HandleHistory(args[1:])
		default:
			fmt.Println("Unknown command. Type 'help'")
		}
	}
}
