package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gitlab.ozon.dev/pupkingeorgij/homework/storage"
	"gitlab.ozon.dev/pupkingeorgij/homework/types"
)

var (
	stg storage.Storage
)

func main() {
	var err error
	stg, err = storage.NewFileStorage("storage.json")
	if err != nil {
		fmt.Println("Error initializing storage:", err)
		return
	}

	fmt.Println("Pick-up point Manager started. Type 'help' for commands list.")

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
		case "help":
			printHelp()
		case "accept":
			handleAccept(args[1:])
		case "return":
			handleReturn(args[1:])
		case "process":
			handleProcess(args[1:])
		case "list-orders":
			handleListOrders(args[1:])
		case "list-returns":
			handleListReturns(args[1:])
		case "history":
			handleHistory(args[1:])
		default:
			fmt.Println("Unknown command. Type 'help' for available commands.")
		}
	}
}

func printHelp() {
	fmt.Println(`Available commands:
    accept <orderID> <recipientID> <storageUntil> - Accept order from courier
    return <orderID> - Return order to courier
    process <userID> <action> <orderIDs...> - Process client orders
    list-orders <userID> [--last N] [--active] - List user orders
    list-returns [page] [limit] - List returns with pagination
    history <orderID> - Get order history
    exit - Exit program`)
}

func handleAccept(args []string) {
	if len(args) != 3 {
		fmt.Println("Usage: accept <orderID> <recipientID> <storageUntil (RFC3339)>")
		return
	}

	storageUntil, err := time.Parse(time.RFC3339, args[2])
	if err != nil {
		fmt.Println("Invalid date format:", err)
		return
	}
	if storageUntil.Before(time.Now()) {
		fmt.Println("Error: storage period is in the past")
		return
	}

	order := types.Order{
		ID:           args[0],
		RecipientID:  args[1],
		StorageUntil: storageUntil,
		Status:       "received",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := stg.AddOrder(order); err != nil {
		fmt.Println("Error accepting order:", err)
	} else {
		fmt.Println("Order accepted successfully")
	}
}

func handleProcess(args []string) {
	if len(args) < 3 {
		fmt.Println("Usage: process <userID> <action> <orderIDs...>")
		return
	}

	userID := args[0]
	action := args[1]
	orderIDs := args[2:]

	for _, orderID := range orderIDs {
		order, err := stg.GetOrder(orderID)
		if err != nil {
			fmt.Printf("Order %s not found\n", orderID)
			return
		}
		if order.RecipientID != userID {
			fmt.Println("All orders must belong to the same user")
			return
		}
	}

	switch action {
	case "issue":
		handleIssueOrders(orderIDs)
	case "return":
		handleAcceptReturns(userID, orderIDs)
	default:
		fmt.Println("Invalid action. Use 'issue' or 'return'")
	}
}

func handleIssueOrders(orderIDs []string) {
	for _, orderID := range orderIDs {
		order, err := stg.GetOrder(orderID)
		if err != nil {
			fmt.Printf("Order %s not found\n", orderID)
			continue
		}

		if order.StorageUntil.Before(time.Now()) {
			fmt.Printf("Order %s expired\n", orderID)
			continue
		}

		if order.Status != "received" {
			fmt.Printf("Order %s already processed\n", orderID)
			continue
		}

		if err := stg.UpdateOrderStatus(orderID, "issued"); err != nil {
			fmt.Printf("Failed to issue order %s: %v\n", orderID, err)
		} else {
			fmt.Printf("Order %s issued successfully\n", orderID)
		}
	}
}

func handleAcceptReturns(userID string, orderIDs []string) {
	for _, orderID := range orderIDs {
		order, err := stg.GetOrder(orderID)
		if err != nil {
			fmt.Printf("Order %s not found\n", orderID)
			continue
		}

		if order.Status != "issued" {
			fmt.Printf("Order %s not issued\n", orderID)
			continue
		}

		if time.Since(order.UpdatedAt) > 48*time.Hour {
			fmt.Printf("Return period expired for order %s\n", orderID)
			continue
		}

		returnEntry := types.Return{
			OrderID:    orderID,
			UserID:     userID,
			ReturnedAt: time.Now(),
		}

		if err := stg.AddReturn(returnEntry); err != nil {
			fmt.Printf("Failed to accept return for order %s: %v\n", orderID, err)
		} else {
			fmt.Printf("Return accepted for order %s\n", orderID)
			stg.UpdateOrderStatus(orderID, "returned")
		}
	}
}

func handleListOrders(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: list-orders <userID> [--last N] [--active]")
		return
	}

	userID := args[0]
	var lastN int
	activeOnly := false

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--last":
			if i+1 >= len(args) {
				fmt.Println("Missing value for --last")
				return
			}
			n, err := strconv.Atoi(args[i+1])
			if err != nil {
				fmt.Println("Invalid value for --last")
				return
			}
			lastN = n
			i++
		case "--active":
			activeOnly = true
		}
	}

	orders, err := stg.GetUserOrders(userID, lastN, activeOnly)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Println("Orders:")
	for _, o := range orders {
		fmt.Printf("- ID: %s, Status: %s, Storage Until: %s\n",
			o.ID, o.Status, o.StorageUntil.Format(time.RFC822))
	}
}

func handleListReturns(args []string) {
	page := 1
	limit := 10

	if len(args) >= 1 {
		p, err := strconv.Atoi(args[0])
		if err == nil {
			page = p
		}
	}
	if len(args) >= 2 {
		l, err := strconv.Atoi(args[1])
		if err == nil {
			limit = l
		}
	}

	returns, err := stg.GetReturns(page, limit)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Printf("Returns (Page %d):\n", page)
	for _, r := range returns {
		fmt.Printf("- Order: %s, User: %s, Returned At: %s\n",
			r.OrderID, r.UserID, r.ReturnedAt.Format(time.RFC822))
	}
}

func handleHistory(args []string) {
	if len(args) != 1 {
		fmt.Println("Usage: history <orderID>")
		return
	}

	history, err := stg.GetOrderHistory(args[0])
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Println("Order history:")
	for _, h := range history {
		fmt.Printf("- %s: %s\n", h.ChangedAt.Format(time.RFC822), h.Status)
	}
}
func handleReturn(args []string) {
	if len(args) != 1 {
		fmt.Println("Usage: return <orderID>")
		return
	}

	orderID := args[0]

	order, err := stg.GetOrder(orderID)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	if order.Status != "received" {
		fmt.Println("Cannot return order: it must be in 'received' status")
		return
	}

	if order.StorageUntil.After(time.Now()) {
		fmt.Println("Cannot return order: storage period has not expired yet")
		return
	}
	if err := stg.UpdateOrderStatus(orderID, "returned_to_courier"); err != nil {
		fmt.Println("Error updating order history:", err)
		return
	}
	if err := stg.DeleteOrder(orderID); err != nil {
		fmt.Println("Error returning order:", err)
		return
	}
	fmt.Println("Order returned to courier successfully")
}
