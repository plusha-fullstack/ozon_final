// server/audit.go
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

type AuditManager struct {
	workerCount int
	batchSize   int
	timeout     time.Duration

	inputChan  chan AuditLogEntry
	batchChan  chan []AuditLogEntry
	shutdownCh chan struct{}
	once       sync.Once

	wg           sync.WaitGroup
	pendingMu    sync.Mutex
	pendingCount int
}

func (m *AuditManager) Shutdown(ctx context.Context) {
	m.once.Do(func() {
		log.Println("Initiating AuditManager shutdown")
		close(m.shutdownCh)

		done := make(chan struct{})
		go func() {
			m.wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			log.Println("AuditManager shutdown completed")
		case <-ctx.Done():
			log.Println("WARNING: AuditManager shutdown interrupted")
		}
	})
}

func (m *AuditManager) monitorShutdown(ctx context.Context) {
	<-ctx.Done()
	log.Println("Context cancellation detected")
	m.Shutdown(context.Background())
}

func NewAuditManager(workerCount, batchSize int, timeout time.Duration) *AuditManager {
	return &AuditManager{
		workerCount: workerCount,
		batchSize:   batchSize,
		timeout:     timeout,
		inputChan:   make(chan AuditLogEntry, workerCount*batchSize*2),
		batchChan:   make(chan []AuditLogEntry, workerCount*2),
		shutdownCh:  make(chan struct{}),
	}
}

func (m *AuditManager) Start(ctx context.Context) {
	log.Println("Starting AuditManager")
	m.wg.Add(1)
	go m.runAggregator(ctx)

	for i := 0; i < m.workerCount; i++ {
		m.wg.Add(1)
		go m.runWorker(ctx, i)
	}

	go m.monitorShutdown(ctx)
}

func (m *AuditManager) LogEntry(ctx context.Context, entry AuditLogEntry) {
	m.pendingMu.Lock()
	m.pendingCount++
	m.pendingMu.Unlock()

	select {
	case m.inputChan <- entry:
	case <-ctx.Done():
		m.emergencyLog(entry)
	}
}

func (m *AuditManager) runAggregator(ctx context.Context) {
	defer m.wg.Done()
	log.Println("Aggregator started")

	var (
		batch    []AuditLogEntry
		timer    *time.Timer
		timeoutC <-chan time.Time
	)

	defer func() {
		if timer != nil {
			timer.Stop()
		}
		if len(batch) > 0 {
			m.dispatchBatch(batch)
		}
		close(m.batchChan)
	}()

	for {
		select {
		case entry, ok := <-m.inputChan:
			if !ok {
				return
			}

			batch = append(batch, entry)
			if len(batch) >= m.batchSize {
				m.dispatchBatch(batch)
				batch = nil
				timeoutC = nil
			} else if len(batch) == 1 {
				timer = time.NewTimer(m.timeout)
				timeoutC = timer.C
			}

		case <-timeoutC:
			m.dispatchBatch(batch)
			batch = nil
			timeoutC = nil

		case <-ctx.Done():
			return

		case <-m.shutdownCh:
			return
		}
	}
}

func (m *AuditManager) dispatchBatch(batch []AuditLogEntry) {
	batchCopy := make([]AuditLogEntry, len(batch))
	copy(batchCopy, batch)

	select {
	case m.batchChan <- batchCopy:
		m.updatePendingCount(-len(batch))
	default:
		m.printBatch(-1, batchCopy)
	}
}

func (m *AuditManager) runWorker(ctx context.Context, id int) {
	defer m.wg.Done()
	log.Printf("Worker %d started", id)

	for {
		select {
		case batch, ok := <-m.batchChan:
			if !ok {
				log.Printf("Worker %d exiting", id)
				return
			}
			m.printBatch(id, batch)
		case <-ctx.Done():
			for {
				select {
				case batch, ok := <-m.batchChan:
					if !ok {
						log.Printf("Worker %d exiting", id)
						return
					}
					m.printBatch(id, batch)
				default:
					log.Printf("Worker %d exiting", id)
					return
				}
			}
		}
	}
}

func (m *AuditManager) emergencyLog(entry AuditLogEntry) {
	entryJSON, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		log.Printf("ERROR: Failed to marshal emergency entry: %v", err)
		return
	}

	fmt.Printf("\n=== EMERGENCY LOG ===\n%s\n=== END LOG ===\n", entryJSON)
	m.updatePendingCount(-1)
}

func (m *AuditManager) printBatch(workerID int, batch []AuditLogEntry) {
	prefix := "DIRECT"
	if workerID >= 0 {
		prefix = fmt.Sprintf("WORKER-%d", workerID)
	}

	fmt.Printf("\n=== BATCH (%s) ===\n", prefix)
	for _, entry := range batch {
		entryJSON, err := json.MarshalIndent(entry, "", "  ")
		if err != nil {
			fmt.Printf("ERROR: %v\n", err)
			continue
		}
		fmt.Println(string(entryJSON))
	}
	fmt.Println("=== END BATCH ===")
}

func (m *AuditManager) updatePendingCount(delta int) {
	m.pendingMu.Lock()
	defer m.pendingMu.Unlock()
	m.pendingCount += delta
}
