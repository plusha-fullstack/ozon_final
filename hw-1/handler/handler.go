package handler

import (
	"fmt"
	"strconv"
	"time"

	"gitlab.ozon.dev/pupkingeorgij/homework/internal/storage"
)

type Storage interface {
	AddOrder(order storage.Order) error
	GetOrder(orderID string) (*storage.Order, error)
	UpdateOrderStatus(orderID, status string) error
	DeleteOrder(orderID string) error
	GetUserOrders(userID string, lastN int, activeOnly bool) ([]storage.Order, error)
	AddReturn(ret storage.Return) error
	GetReturns(page, limit int) ([]storage.Return, error)
	GetOrderHistory(orderID string) ([]storage.HistoryEntry, error)
}

type Handler struct {
	storage Storage
}

func New(storage Storage) *Handler {
	return &Handler{storage: storage}
}

func (h *Handler) HandleHelp() {
	fmt.Println(`Available commands:
	accept <orderID> <recipientID> <YYYY-MM-DD> - Accept order
	return <orderID> - Return order to courier
	process <userID> <issue|return> <orderID...> - Process client order
	list-orders <userID> [--last N] [--active] - List orders
	list-returns [page] [limit] - List returns
	history <orderID> - Show order history
	exit - Exit program`)
}

func (h *Handler) HandleAccept(args []string) {
	if len(args) != 3 {
		fmt.Println("Usage: accept <orderID> <recipientID> <YYYY-MM-DD>")
		return
	}

	date, err := time.Parse("2006-01-02", args[2])
	if err != nil {
		fmt.Println("Invalid date format. Use YYYY-MM-DD")
		return
	}
	if date.Before(time.Now()) {
		fmt.Println("Error: storage period is in the past")
		return
	}

	order := storage.Order{
		ID:           args[0],
		RecipientID:  args[1],
		StorageUntil: date.UTC(),
		Status:       "received",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	if err := h.storage.AddOrder(order); err != nil {
		fmt.Println("Error:", err)
	} else {
		fmt.Println("Order accepted successfully")
	}
}

func (h *Handler) HandleReturn(args []string) {
	if len(args) != 1 {
		fmt.Println("Usage: return <orderID>")
		return
	}

	order, err := h.storage.GetOrder(args[0])
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	if order.Status != "received" {
		fmt.Println("Cannot return: order not in 'received' status")
		return
	}

	if time.Now().UTC().Before(order.StorageUntil) {
		fmt.Println("Cannot return: storage period not expired")
		return
	}

	if err := h.storage.DeleteOrder(args[0]); err != nil {
		fmt.Println("Error:", err)
	} else {
		fmt.Println("Order returned to courier")
	}
}

func (h *Handler) HandleProcess(args []string) {
	if len(args) < 3 {
		fmt.Println("Usage: process <userID> <issue|return> <orderID...>")
		return
	}

	userID := args[0]
	action := args[1]
	orderIDs := args[2:]

	switch action {
	case "issue":
		h.handleIssueOrders(userID, orderIDs)
	case "return":
		h.handleAcceptReturns(userID, orderIDs)
	default:
		fmt.Println("Invalid action. Use 'issue' or 'return'")
	}
}

func (h *Handler) handleIssueOrders(userID string, orderIDs []string) {
	for _, orderID := range orderIDs {
		order, err := h.storage.GetOrder(orderID)
		if err != nil {
			fmt.Printf("Error: order %s not found\n", orderID)
			continue
		}

		if order.RecipientID != userID {
			fmt.Printf("Error: order %s does not belong to user %s\n", orderID, userID)
			continue
		}

		if order.Status != "received" {
			fmt.Printf("Error: order %s is not in 'received' status\n", orderID)
			continue
		}

		if time.Now().UTC().After(order.StorageUntil) {
			fmt.Printf("Error: order %s has expired\n", orderID)
			continue
		}

		if err := h.storage.UpdateOrderStatus(orderID, "issued"); err != nil {
			fmt.Printf("Error: failed to issue order %s\n", orderID)
		} else {
			fmt.Printf("Order %s issued successfully\n", orderID)
		}
	}
}

func (h *Handler) handleAcceptReturns(userID string, orderIDs []string) {
	for _, orderID := range orderIDs {
		order, err := h.storage.GetOrder(orderID)
		if err != nil {
			fmt.Printf("Error: order %s not found\n", orderID)
			continue
		}

		if order.RecipientID != userID {
			fmt.Printf("Error: order %s does not belong to user %s\n", orderID, userID)
			continue
		}

		if order.Status != "issued" {
			fmt.Printf("Error: order %s is not in 'issued' status\n", orderID)
			continue
		}

		if time.Since(order.UpdatedAt) > 48*time.Hour {
			fmt.Printf("Error: return period expired for order %s\n", orderID)
			continue
		}

		if err := h.storage.AddReturn(storage.Return{
			OrderID:    orderID,
			UserID:     userID,
			ReturnedAt: time.Now().UTC(),
		}); err != nil {
			fmt.Printf("Error: failed to accept return for order %s\n", orderID)
		} else {
			fmt.Printf("Return accepted for order %s\n", orderID)
			h.storage.UpdateOrderStatus(orderID, "returned")
		}
	}
}

func (h *Handler) HandleListOrders(args []string) {
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
			if err != nil || n <= 0 {
				fmt.Println("Invalid value for --last")
				return
			}
			lastN = n
			i++
		case "--active":
			activeOnly = true
		}
	}

	orders, err := h.storage.GetUserOrders(userID, lastN, activeOnly)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	if len(orders) == 0 {
		fmt.Println("No orders found")
		return
	}

	fmt.Println("Orders:")
	for _, o := range orders {
		status := fmt.Sprintf("[%s]", o.Status)
		if o.StorageUntil.Before(time.Now().UTC()) {
			status += " (expired)"
		}
		fmt.Printf("- %s | Recipient: %s | Storage until: %s %s\n",
			o.ID, o.RecipientID, o.StorageUntil.Format("2006-01-02"), status)
	}
}

func (h *Handler) HandleListReturns(args []string) {
	page := 1
	limit := 10

	if len(args) >= 1 {
		p, err := strconv.Atoi(args[0])
		if err == nil && p > 0 {
			page = p
		}
	}
	if len(args) >= 2 {
		l, err := strconv.Atoi(args[1])
		if err == nil && l > 0 {
			limit = l
		}
	}

	returns, err := h.storage.GetReturns(page, limit)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	if len(returns) == 0 {
		fmt.Println("No returns found")
		return
	}

	fmt.Printf("Returns (Page %d):\n", page)
	for _, r := range returns {
		fmt.Printf("- Order: %s | User: %s | Returned At: %s\n",
			r.OrderID, r.UserID, r.ReturnedAt.Format("2006-01-02 15:04:05"))
	}
}

func (h *Handler) HandleHistory(args []string) {
	if len(args) != 1 {
		fmt.Println("Usage: history <orderID>")
		return
	}

	history, err := h.storage.GetOrderHistory(args[0])
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	if len(history) == 0 {
		fmt.Println("No history found for this order")
		return
	}

	fmt.Println("Order history:")
	for _, h := range history {
		fmt.Printf("- %s: %s\n", h.ChangedAt.Format("2006-01-02 15:04:05"), h.Status)
	}
}
