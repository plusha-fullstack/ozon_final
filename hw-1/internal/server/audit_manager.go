package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type AuditManager struct {
	batchSize int
	timeout   time.Duration

	inputChan chan AuditLogEntry
	batchChan chan []AuditLogEntry

	workerCount int

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	pendingEntries int
	mutex          sync.Mutex
}

func NewAuditManager(workerCount, batchSize int, timeout time.Duration) *AuditManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &AuditManager{
		workerCount: workerCount,
		batchSize:   batchSize,
		timeout:     timeout,
		inputChan:   make(chan AuditLogEntry, workerCount*batchSize*2),
		batchChan:   make(chan []AuditLogEntry, workerCount*2),
		ctx:         ctx,
		cancel:      cancel,
	}
}

func (m *AuditManager) Start() {
	log.Println("Starting AuditManager")
	m.wg.Add(1)
	go m.aggregator()

	for i := 0; i < m.workerCount; i++ {
		m.wg.Add(1)
		go m.worker(i)
	}

	go m.setupSignalHandler()
}

func (m *AuditManager) setupSignalHandler() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	sig := <-signals
	log.Printf("Received signal %v, initiating graceful shutdown...", sig)

	m.Shutdown()
}

func (m *AuditManager) LogEntry(entry AuditLogEntry) {
	m.mutex.Lock()
	m.pendingEntries++
	m.mutex.Unlock()

	select {
	case m.inputChan <- entry:
	case <-m.ctx.Done():
		log.Printf("WARNING: AuditManager shutting down, logging entry directly: %+v", entry)
		m.emergencyLog(entry)
	}
}

func (m *AuditManager) emergencyLog(entry AuditLogEntry) {
	entryJSON, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		log.Printf("ERROR: Failed to marshal emergency audit entry: %v", err)
		return
	}

	fmt.Printf("\n=== EMERGENCY AUDIT LOG ENTRY ===\n")
	fmt.Println(string(entryJSON))
	fmt.Println("=== END EMERGENCY ENTRY ===")

	m.mutex.Lock()
	m.pendingEntries--
	m.mutex.Unlock()
}

func (m *AuditManager) aggregator() {
	defer m.wg.Done()
	log.Println("Audit aggregator started")

	var batch []AuditLogEntry
	timer := time.NewTimer(m.timeout)
	if !timer.Stop() {
		<-timer.C
	}

	for {
		if len(batch) == 0 {
			select {
			case entry, ok := <-m.inputChan:
				if !ok {
					log.Println("Input channel closed, flushing final batch")
					if len(batch) > 0 {
						m.dispatchBatch(batch)
					}
					close(m.batchChan)
					return
				}
				batch = append(batch, entry)
				timer.Reset(m.timeout)
			case <-m.ctx.Done():
				log.Println("Context canceled, flushing final batch in aggregator")
				if len(batch) > 0 {
					m.dispatchBatch(batch)
				}
				m.drainRemainingEntries()

				close(m.batchChan)
				return
			}
		} else {
			select {
			case entry, ok := <-m.inputChan:
				if !ok {
					log.Println("Input channel closed during batch processing, flushing batch")
					m.dispatchBatch(batch)
					close(m.batchChan)
					return
				}
				batch = append(batch, entry)
				if len(batch) >= m.batchSize {
					if !timer.Stop() {
						<-timer.C
					}
					m.dispatchBatch(batch)
					batch = nil
				}
			case <-timer.C:
				m.dispatchBatch(batch)
				batch = nil
			case <-m.ctx.Done():
				log.Println("Context canceled during batch processing, flushing batch")
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				if len(batch) > 0 {
					m.dispatchBatch(batch)
				}
				m.drainRemainingEntries()

				close(m.batchChan)
				return
			}
		}
	}
}

func (m *AuditManager) drainRemainingEntries() {
	log.Println("Draining remaining entries from input channel")

	var finalBatch []AuditLogEntry

	draining := true
	for draining {
		select {
		case entry, ok := <-m.inputChan:
			if !ok {
				draining = false
				break
			}
			finalBatch = append(finalBatch, entry)

			if len(finalBatch) >= m.batchSize {
				m.dispatchBatch(finalBatch)
				finalBatch = nil
			}
		default:
			draining = false
		}
	}

	if len(finalBatch) > 0 {
		m.dispatchBatch(finalBatch)
	}
}

func (m *AuditManager) dispatchBatch(batch []AuditLogEntry) {
	if len(batch) == 0 {
		return
	}

	batchCopy := make([]AuditLogEntry, len(batch))
	copy(batchCopy, batch)

	select {
	case m.batchChan <- batchCopy:
		log.Printf("Dispatched batch of %d entries", len(batchCopy))
	case <-m.ctx.Done():
		log.Printf("Context canceled during dispatch, processing batch directly")
		m.printBatch(-1, batchCopy)
	}

	m.mutex.Lock()
	m.pendingEntries -= len(batch)
	m.mutex.Unlock()
}

func (m *AuditManager) worker(workerID int) {
	defer m.wg.Done()
	log.Printf("Audit worker %d started", workerID)

	for {
		select {
		case batch, ok := <-m.batchChan:
			if !ok {
				log.Printf("Batch channel closed, worker %d exiting", workerID)
				return
			}
			m.printBatch(workerID, batch)
		case <-m.ctx.Done():
			log.Printf("Context canceled, worker %d exiting", workerID)
			return
		}
	}
}

func (m *AuditManager) printBatch(workerID int, batch []AuditLogEntry) {
	workerLabel := "DIRECT"
	if workerID >= 0 {
		workerLabel = fmt.Sprintf("Worker %d", workerID)
	}

	fmt.Printf("\n=== AUDIT LOG BATCH (%s) ===\n", workerLabel)
	for _, entry := range batch {
		if entryJSON, err := json.MarshalIndent(entry, "", "  "); err == nil {
			fmt.Println(string(entryJSON))
		} else {
			log.Printf("ERROR: Failed to marshal audit entry: %v", err)
		}
	}
	fmt.Println("=== END BATCH ===")
}

func (m *AuditManager) GetPendingEntryCount() int {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.pendingEntries
}

func (m *AuditManager) Shutdown() {
	log.Println("Initiating AuditManager shutdown")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	m.cancel()

	done := make(chan struct{})
	go func() {
		pendingCount := m.GetPendingEntryCount()
		if pendingCount > 0 {
			log.Printf("Waiting for %d pending audit entries to be processed...", pendingCount)
		}
		close(m.inputChan)
		m.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("AuditManager shutdown completed successfully")
	case <-shutdownCtx.Done():
		log.Println("WARNING: AuditManager shutdown timed out, some entries may be lost")
	}
}
