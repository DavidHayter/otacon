package intelligence

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// CooldownConfig configures the cooldown behavior
type CooldownConfig struct {
	DefaultDuration time.Duration            // Default cooldown period (default 60min)
	PerSeverity     map[string]time.Duration // Override by severity: "critical"=15m, "warning"=60m
	PerGroup        bool                     // Cooldown per group key (vs global)
	MaxSuppressed   int                      // Max suppressed before forcing through (default 100)
}

// DefaultCooldownConfig returns sensible defaults
func DefaultCooldownConfig() CooldownConfig {
	return CooldownConfig{
		DefaultDuration: 60 * time.Minute,
		PerSeverity: map[string]time.Duration{
			"CRITICAL": 15 * time.Minute,
			"WARNING":  60 * time.Minute,
			"INFO":     120 * time.Minute,
		},
		PerGroup:      true,
		MaxSuppressed: 100,
	}
}

// cooldownEntry tracks a single cooldown state
type cooldownEntry struct {
	LastNotified time.Time
	Suppressed   int
}

// CooldownManager manages notification throttling
type CooldownManager struct {
	config    CooldownConfig
	cooldowns map[string]*cooldownEntry
	mu        sync.Mutex
	stats     CooldownStats
}

// CooldownStats tracks cooldown statistics
type CooldownStats struct {
	TotalReceived  int64
	TotalPassed    int64
	TotalSuppressed int64
}

// NewCooldownManager creates a new cooldown manager
func NewCooldownManager(config CooldownConfig) *CooldownManager {
	return &CooldownManager{
		config:    config,
		cooldowns: make(map[string]*cooldownEntry),
	}
}

// ShouldNotify checks if a notification should be sent for the given key and severity
// Returns true if notification should proceed, false if suppressed
func (c *CooldownManager) ShouldNotify(key string, severity string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.stats.TotalReceived++

	// Determine cooldown duration
	duration := c.config.DefaultDuration
	if d, ok := c.config.PerSeverity[severity]; ok {
		duration = d
	}

	// Build lookup key
	lookupKey := key
	if !c.config.PerGroup {
		lookupKey = "global"
	}

	entry, exists := c.cooldowns[lookupKey]
	if !exists {
		// First notification — allow and start cooldown
		c.cooldowns[lookupKey] = &cooldownEntry{
			LastNotified: time.Now(),
			Suppressed:   0,
		}
		c.stats.TotalPassed++
		return true
	}

	elapsed := time.Since(entry.LastNotified)

	// Cooldown expired — allow
	if elapsed >= duration {
		entry.LastNotified = time.Now()
		if entry.Suppressed > 0 {
			log.Printf("[cooldown] Key '%s': cooldown expired, %d events were suppressed", lookupKey, entry.Suppressed)
		}
		entry.Suppressed = 0
		c.stats.TotalPassed++
		return true
	}

	// Force through if too many suppressed (safety valve)
	if entry.Suppressed >= c.config.MaxSuppressed {
		entry.LastNotified = time.Now()
		log.Printf("[cooldown] Key '%s': safety valve triggered after %d suppressed events", lookupKey, entry.Suppressed)
		entry.Suppressed = 0
		c.stats.TotalPassed++
		return true
	}

	// Suppress
	entry.Suppressed++
	c.stats.TotalSuppressed++
	return false
}

// Reset clears the cooldown for a specific key
func (c *CooldownManager) Reset(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.cooldowns, key)
}

// ResetAll clears all cooldowns
func (c *CooldownManager) ResetAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cooldowns = make(map[string]*cooldownEntry)
	log.Println("[cooldown] All cooldowns reset")
}

// GetStats returns cooldown statistics
func (c *CooldownManager) GetStats() CooldownStats {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stats
}

// Cleanup removes expired cooldown entries
func (c *CooldownManager) Cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Remove entries older than 2x the default duration
	cutoff := time.Now().Add(-c.config.DefaultDuration * 2)
	for key, entry := range c.cooldowns {
		if entry.LastNotified.Before(cutoff) {
			delete(c.cooldowns, key)
		}
	}
}

// Status returns a human-readable status string
func (c *CooldownManager) Status() string {
	c.mu.Lock()
	defer c.mu.Unlock()

	active := 0
	totalSuppressed := 0
	for _, entry := range c.cooldowns {
		if time.Since(entry.LastNotified) < c.config.DefaultDuration {
			active++
			totalSuppressed += entry.Suppressed
		}
	}

	return fmt.Sprintf("Active cooldowns: %d, Currently suppressed: %d, Total passed: %d, Total suppressed: %d",
		active, totalSuppressed, c.stats.TotalPassed, c.stats.TotalSuppressed)
}
