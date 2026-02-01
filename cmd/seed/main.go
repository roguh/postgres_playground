package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"roguh.com/postgres_playground/pkg/database"
)

// Realistic messy JSON generators
func genSiteMetadata() json.RawMessage {
	templates := []string{
		// Old format from legacy system
		`{"type":"%s","manager":"%s","phone":"%s","legacy_id":%d,"active":true}`,
		// New format with nested structure
		`{"facility":{"type":"%s","classification":"%s"},"contact":{"name":"%s","email":"%s","phone":%s},"compliance":{"certifications":[%s],"last_audit":"%s"}}`,
		// Inconsistent format mixing conventions
		`{"facilityType":"%s","Manager":"%s","contact_phone":"%s","metadata":{"source":"import_2019","verified":%t},"tags":[%s]}`,
		// Ultra nested with arrays
		`{"operations":{"schedule":{"timezone":"%s","hours":[%s]},"staff_count":%d},"systems":[%s],"notes":"%s","_internal":{"migration_version":2}}`,
	}

	switch rand.Intn(4) {
	case 0:
		return json.RawMessage(fmt.Sprintf(templates[0],
			randomFrom("warehouse", "retail", "office", "datacenter"),
			randomName(),
			randomPhone(),
			rand.Intn(99999)))
	case 1:
		certs := []string{}
		for i := 0; i < rand.Intn(4); i++ {
			certs = append(certs, fmt.Sprintf(`"%s"`, randomFrom("ISO9001", "ISO27001", "SOC2", "HIPAA", "PCI-DSS")))
		}
		return json.RawMessage(fmt.Sprintf(templates[1],
			randomFrom("warehouse", "retail", "office", "datacenter"),
			randomFrom("A", "B", "C", "Critical"),
			randomName(),
			randomEmail(),
			randomPhoneJSON(),
			strings.Join(certs, ","),
			time.Now().AddDate(0, -rand.Intn(12), 0).Format(time.RFC3339)))
	case 2:
		tags := []string{}
		for i := 0; i < rand.Intn(5); i++ {
			tags = append(tags, fmt.Sprintf(`"%s"`, randomFrom("priority", "24x7", "remote", "unstaffed", "construction")))
		}
		return json.RawMessage(fmt.Sprintf(templates[2],
			randomFrom("WAREHOUSE", "RETAIL", "OFFICE", "DC"),
			randomName(),
			randomPhone(),
			rand.Float32() > 0.5,
			strings.Join(tags, ",")))
	default:
		hours := []string{}
		days := []string{"mon", "tue", "wed", "thu", "fri"}
		for _, day := range days {
			hours = append(hours, fmt.Sprintf(`{"day":"%s","open":"08:00","close":"%02d:00"}`, day, 17+rand.Intn(3)))
		}
		systems := []string{}
		for i := 0; i < rand.Intn(3)+1; i++ {
			systems = append(systems, fmt.Sprintf(`{"type":"%s","vendor":"%s","version":"%d.%d"}`,
				randomFrom("HVAC", "Security", "Power", "Network"),
				randomFrom("Honeywell", "Schneider", "Siemens", "Johnson"),
				rand.Intn(5)+1, rand.Intn(20)))
		}
		return json.RawMessage(fmt.Sprintf(templates[3],
			randomFrom("America/New_York", "America/Chicago", "America/Los_Angeles", "UTC"),
			strings.Join(hours, ","),
			rand.Intn(50)+10,
			strings.Join(systems, ","),
			randomFrom("Scheduled for renovation", "New tenant incoming", "Expansion planned", "")))
	}
}

func genAssetConfig() json.RawMessage {
	templates := []string{
		// Simple flat config
		`{"ip":"%s","subnet":"%s","gateway":"%s","dns":["%s","%s"]}`,
		// Nested with arrays and nulls
		`{"network":{"interfaces":[{"name":"eth0","ip":"%s","mac":"%s"},{"name":"eth1","ip":"%s","status":"%s"}]},"snmp":{"version":%d,"community":%s}}`,
		// Mixed types and inconsistent naming
		`{"IP_ADDRESS":"%s","firmware":{"current":"%s","available":%s},"settings":{"power_mode":"%s","temp_threshold":%d},"custom_fields":%s}`,
		// Deep nesting with conditional fields
		`{"provisioning":{"method":"%s","server":%s,"profile":"%s"},"features":{%s},"debug":{"enabled":%t,"level":%d,"output":%s}}`,
	}

	switch rand.Intn(4) {
	case 0:
		return json.RawMessage(fmt.Sprintf(templates[0],
			randomIP(), "255.255.255.0", randomIP(), "8.8.8.8", "8.8.4.4"))
	case 1:
		community := "null"
		if rand.Float32() > 0.3 {
			community = fmt.Sprintf(`"%s"`, randomFrom("public", "private", "monitor"))
		}
		return json.RawMessage(fmt.Sprintf(templates[1],
			randomIP(), randomMAC(), randomIP(),
			randomFrom("up", "down", "unknown"),
			randomFrom(1, 2, 3),
			community))
	case 2:
		available := "null"
		if rand.Float32() > 0.5 {
			available = fmt.Sprintf(`"%d.%d.%d"`, rand.Intn(3)+1, rand.Intn(10), rand.Intn(100))
		}
		customFields := "{}"
		if rand.Float32() > 0.6 {
			customFields = fmt.Sprintf(`{"dept":"%s","cost_center":%d,"labels":[%s]}`,
				randomFrom("IT", "OPS", "SALES", "HR"),
				rand.Intn(9999),
				fmt.Sprintf(`"%s","%s"`, randomFrom("critical", "prod", "test"), randomFrom("managed", "unmanaged")))
		}
		return json.RawMessage(fmt.Sprintf(templates[2],
			randomIP(),
			fmt.Sprintf("%d.%d.%d", rand.Intn(3)+1, rand.Intn(10), rand.Intn(100)),
			available,
			randomFrom("normal", "eco", "performance"),
			rand.Intn(40)+60,
			customFields))
	default:
		server := "null"
		if rand.Float32() > 0.4 {
			server = fmt.Sprintf(`"%s"`, randomFrom("pxe.local", "config.corp.net", "10.0.0.5"))
		}
		features := []string{}
		possibleFeatures := []string{"monitoring", "alerting", "remote_access", "auto_update", "telemetry"}
		for _, f := range possibleFeatures {
			if rand.Float32() > 0.5 {
				features = append(features, fmt.Sprintf(`"%s":{"enabled":%t,"config":%s}`,
					f, rand.Float32() > 0.3,
					randomFrom(`{}`, `{"interval":300}`, `{"threshold":0.8}`)))
			}
		}
		output := randomFrom(`"syslog"`, `"file"`, `["console","syslog"]`, "null")
		return json.RawMessage(fmt.Sprintf(templates[3],
			randomFrom("dhcp", "static", "pxe"),
			server,
			randomFrom("default", "secure", "performance", "minimal"),
			strings.Join(features, ","),
			rand.Float32() > 0.7,
			rand.Intn(5),
			output))
	}
}

func genAssetTelemetry() json.RawMessage {
	// Simulate real-world messy telemetry data
	templates := []string{
		// Simple metrics
		`{"cpu":%d,"memory":%d,"disk":%d,"uptime":%d}`,
		// Nested with timestamps
		`{"metrics":{"cpu":{"value":%f,"unit":"percent","timestamp":"%s"},"memory":{"used":%d,"total":%d,"unit":"MB"},"temp":{"value":%f,"unit":"%s","sensor":"%s"}},"errors":%d}`,
		// Array-based time series (last N readings)
		`{"readings":[%s],"summary":{"avg_cpu":%f,"max_memory":%d,"alerts":[%s]},"device_time":"%s"}`,
		// Mixed formats from different firmware versions
		`{"v1_format":{"cpu_usage":%d,"mem_free":%d},"v2_format":{"system":{"processor":{"usage":%f,"cores":%d},"memory":{"available_gb":%f}}},"collection_errors":[%s]}`,
	}

	switch rand.Intn(4) {
	case 0:
		return json.RawMessage(fmt.Sprintf(templates[0],
			rand.Intn(100), rand.Intn(100), rand.Intn(100), rand.Intn(86400*30)))
	case 1:
		return json.RawMessage(fmt.Sprintf(templates[1],
			rand.Float32()*100,
			time.Now().Add(-time.Duration(rand.Intn(3600))*time.Second).Format(time.RFC3339),
			rand.Intn(32768), 32768,
			rand.Float32()*40+20,
			randomFrom("celsius", "C", "fahrenheit"),
			randomFrom("cpu", "ambient", "chassis"),
			rand.Intn(10)))
	case 2:
		readings := []string{}
		for i := 0; i < rand.Intn(5)+3; i++ {
			readings = append(readings, fmt.Sprintf(`{"ts":"%s","cpu":%f,"mem":%d}`,
				time.Now().Add(-time.Duration(i*5)*time.Minute).Format(time.RFC3339),
				rand.Float32()*100,
				rand.Intn(100)))
		}
		alerts := []string{}
		if rand.Float32() > 0.7 {
			alerts = append(alerts, fmt.Sprintf(`{"type":"%s","time":"%s","severity":%d}`,
				randomFrom("high_cpu", "memory_pressure", "disk_full"),
				time.Now().Add(-time.Duration(rand.Intn(3600))*time.Second).Format(time.RFC3339),
				rand.Intn(3)+1))
		}
		return json.RawMessage(fmt.Sprintf(templates[2],
			strings.Join(readings, ","),
			rand.Float32()*100,
			rand.Intn(32768),
			strings.Join(alerts, ","),
			time.Now().Format(time.RFC3339)))
	default:
		errors := []string{}
		if rand.Float32() > 0.8 {
			errors = append(errors, fmt.Sprintf(`{"sensor":"%s","error":"%s","count":%d}`,
				randomFrom("disk_smart", "network_stats", "power_consumption"),
				randomFrom("timeout", "invalid_response", "sensor_offline"),
				rand.Intn(100)+1))
		}
		return json.RawMessage(fmt.Sprintf(templates[3],
			rand.Intn(100), rand.Intn(16384),
			rand.Float32()*100, randomFrom(1, 2, 4, 8),
			float32(rand.Intn(16384))/1024,
			strings.Join(errors, ",")))
	}
}

// Seed functions
func seedSites(ctx context.Context, pool *database.Pool, count int) error {
	log.Printf("Seeding %d sites...", count)

	countries := []string{"US", "CA", "GB", "DE", "FR", "JP", "AU", "BR"}
	cities := map[string][]string{
		"US": {"New York", "Los Angeles", "Chicago", "Houston", "Phoenix"},
		"CA": {"Toronto", "Vancouver", "Montreal", "Calgary", "Ottawa"},
		"GB": {"London", "Manchester", "Birmingham", "Glasgow", "Liverpool"},
		"DE": {"Berlin", "Munich", "Hamburg", "Cologne", "Frankfurt"},
		"FR": {"Paris", "Lyon", "Marseille", "Toulouse", "Nice"},
		"JP": {"Tokyo", "Osaka", "Kyoto", "Yokohama", "Nagoya"},
		"AU": {"Sydney", "Melbourne", "Brisbane", "Perth", "Adelaide"},
		"BR": {"São Paulo", "Rio de Janeiro", "Brasília", "Salvador", "Fortaleza"},
	}

	batch := &pgx.Batch{}
	for i := 0; i < count; i++ {
		country := countries[rand.Intn(len(countries))]
		city := cities[country][rand.Intn(len(cities[country]))]

		// Some sites have coordinates, some don't (real world messiness)
		var lat, lon *float64
		if rand.Float32() > 0.2 {
			latVal := rand.Float64()*180 - 90
			lonVal := rand.Float64()*360 - 180
			lat, lon = &latVal, &lonVal
		}

		query := `
			INSERT INTO sites (name, address, city, country, coordinates, metadata)
			VALUES ($1, $2, $3, $4, point($5, $6), $7)
		`
		batch.Queue(query,
			fmt.Sprintf("%s Site %d", city, i+1),
			fmt.Sprintf("%d %s Street", rand.Intn(9999)+1, randomFrom("Main", "First", "Park", "Oak", "Elm")),
			city,
			country,
			lat, lon,
			genSiteMetadata())

		// Execute in batches
		if batch.Len() >= 100 {
			br := pool.SendBatch(ctx, batch)
			if err := br.Close(); err != nil {
				return fmt.Errorf("batch insert sites: %w", err)
			}
			batch = &pgx.Batch{}
		}
	}

	// Final batch
	if batch.Len() > 0 {
		br := pool.SendBatch(ctx, batch)
		if err := br.Close(); err != nil {
			return fmt.Errorf("final batch insert sites: %w", err)
		}
	}

	log.Printf("✓ Seeded %d sites", count)
	return nil
}

func seedAssets(ctx context.Context, pool *database.Pool, count int) error {
	log.Printf("Seeding %d assets...", count)

	// Get site IDs
	rows, err := pool.Query(ctx, "SELECT id FROM sites")
	if err != nil {
		return fmt.Errorf("query sites: %w", err)
	}
	defer rows.Close()

	var siteIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return err
		}
		siteIDs = append(siteIDs, id)
	}

	if len(siteIDs) == 0 {
		return fmt.Errorf("no sites found")
	}

	assetTypes := []string{"router", "switch", "server", "sensor", "camera", "ups", "hvac", "generator"}
	manufacturers := []string{"Cisco", "Dell", "HP", "Ubiquiti", "APC", "Panduit", "Honeywell"}
	statuses := []string{"active", "active", "active", "active", "maintenance", "offline", "retired"}

	batch := &pgx.Batch{}
	for i := 0; i < count; i++ {
		assetType := assetTypes[rand.Intn(len(assetTypes))]
		manufacturer := manufacturers[rand.Intn(len(manufacturers))]

		// Vary last_seen to simulate real-world scenarios
		lastSeen := time.Now()
		if rand.Float32() > 0.8 {
			lastSeen = lastSeen.Add(-time.Duration(rand.Intn(72)) * time.Hour)
		}

		query := `
			INSERT INTO assets (
				site_id, mac_address, serial_number, asset_type,
				manufacturer, model, firmware_version, status,
				config, telemetry, last_seen
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		`
		batch.Queue(query,
			siteIDs[rand.Intn(len(siteIDs))],
			randomMAC(),
			fmt.Sprintf("%s%d%s", manufacturer[:3], time.Now().Unix(), rand.Intn(99999)),
			assetType,
			manufacturer,
			fmt.Sprintf("%s-%d", assetType, rand.Intn(9999)),
			fmt.Sprintf("%d.%d.%d", rand.Intn(5)+1, rand.Intn(20), rand.Intn(100)),
			statuses[rand.Intn(len(statuses))],
			genAssetConfig(),
			genAssetTelemetry(),
			lastSeen)

		// Execute in batches
		if batch.Len() >= 100 {
			br := pool.SendBatch(ctx, batch)
			if err := br.Close(); err != nil {
				return fmt.Errorf("batch insert assets: %w", err)
			}
			batch = &pgx.Batch{}
		}
	}

	// Final batch
	if batch.Len() > 0 {
		br := pool.SendBatch(ctx, batch)
		if err := br.Close(); err != nil {
			return fmt.Errorf("final batch insert assets: %w", err)
		}
	}

	log.Printf("✓ Seeded %d assets", count)
	return nil
}

// Helper functions
func randomFrom(options ...interface{}) interface{} {
	return options[rand.Intn(len(options))]
}

func randomName() string {
	first := []string{"John", "Jane", "Bob", "Alice", "Charlie", "Diana", "Frank", "Grace"}
	last := []string{"Smith", "Johnson", "Williams", "Brown", "Jones", "Garcia", "Miller", "Davis"}
	return fmt.Sprintf("%s %s", first[rand.Intn(len(first))], last[rand.Intn(len(last))])
}

func randomEmail() string {
	return fmt.Sprintf("%s@example.com", strings.ToLower(strings.Replace(randomName(), " ", ".", 1)))
}

func randomPhone() string {
	return fmt.Sprintf("+1-%d-%d-%d", rand.Intn(899)+100, rand.Intn(899)+100, rand.Intn(8999)+1000)
}

func randomPhoneJSON() string {
	if rand.Float32() > 0.8 {
		return "null"
	}
	return fmt.Sprintf(`"%s"`, randomPhone())
}

func randomIP() string {
	return fmt.Sprintf("10.%d.%d.%d", rand.Intn(256), rand.Intn(256), rand.Intn(256))
}

func randomMAC() string {
	return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x",
		rand.Intn(256), rand.Intn(256), rand.Intn(256),
		rand.Intn(256), rand.Intn(256), rand.Intn(256))
}

func main() {
	rand.Seed(time.Now().UnixNano())

	ctx := context.Background()

	// Connect to database
	pool, err := database.NewPool(ctx, database.DefaultConfig())
	if err != nil {
		log.Fatal("Failed to create pool:", err)
	}
	defer pool.Close()

	// Check if already seeded
	var count int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM sites").Scan(&count)
	if err != nil {
		log.Fatal("Failed to check existing data:", err)
	}

	if count > 0 {
		log.Printf("Database already contains %d sites. Clear data first if you want to reseed.", count)
		return
	}

	// Seed data
	if err := seedSites(ctx, pool, 1000); err != nil {
		log.Fatal("Failed to seed sites:", err)
	}

	if err := seedAssets(ctx, pool, 100000); err != nil {
		log.Fatal("Failed to seed assets:", err)
	}

	// Print statistics
	var siteCount, assetCount int
	pool.QueryRow(ctx, "SELECT COUNT(*) FROM sites").Scan(&siteCount)
	pool.QueryRow(ctx, "SELECT COUNT(*) FROM assets").Scan(&assetCount)

	log.Printf("\n✅ Seeding complete!")
	log.Printf("   Sites:  %d", siteCount)
	log.Printf("   Assets: %d", assetCount)
	log.Printf("   Avg assets per site: %.1f", float64(assetCount)/float64(siteCount))
}
