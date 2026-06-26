package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL not set")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	// Show current state
	fmt.Println("=== BEFORE ===")
	rows, _ := pool.Query(ctx, "SELECT slug, ats_platform, name FROM companies ORDER BY name")
	for rows.Next() {
		var slug, ats, name string
		rows.Scan(&slug, &ats, &name)
		fmt.Printf("  %-20s %-12s %s\n", slug, ats, name)
	}
	rows.Close()

	// 1. Update Netlify: lever -> greenhouse
	tag, err := pool.Exec(ctx,
		`UPDATE companies SET ats_platform = 'greenhouse' WHERE slug = 'netlify' AND ats_platform = 'lever'`)
	fmt.Printf("\n[1] Netlify lever->greenhouse: %s (err=%v)\n", tag, err)

	// 2. Update Notion: greenhouse -> ashby
	tag, err = pool.Exec(ctx,
		`UPDATE companies SET ats_platform = 'ashby' WHERE slug = 'notion' AND ats_platform = 'greenhouse'`)
	fmt.Printf("[2] Notion greenhouse->ashby: %s (err=%v)\n", tag, err)

	// 3. Replace Shopify with Discord
	//    First delete any old jobs for shopify, then update the company row
	tag, err = pool.Exec(ctx,
		`DELETE FROM jobs WHERE company_id = (SELECT id FROM companies WHERE slug = 'shopify')`)
	fmt.Printf("[3a] Deleted shopify jobs: %s (err=%v)\n", tag, err)

	tag, err = pool.Exec(ctx,
		`UPDATE companies 
		 SET name = 'Discord', slug = 'discord', website = 'https://discord.com',
		     ats_platform = 'greenhouse', careers_url = 'https://discord.com/careers',
		     industry = 'Social / Communication', updated_at = NOW()
		 WHERE slug = 'shopify'`)
	fmt.Printf("[3b] Shopify->Discord: %s (err=%v)\n", tag, err)

	// 4. If Notion didn't exist at all (step 2 updated 0 rows), insert it as Ashby
	tag, err = pool.Exec(ctx,
		`INSERT INTO companies (name, slug, website, ats_platform, careers_url, industry)
		 VALUES ('Notion', 'notion', 'https://notion.so', 'ashby', 'https://notion.so/careers', 'Productivity')
		 ON CONFLICT (slug) DO NOTHING`)
	fmt.Printf("[4] Ensure Notion exists (ashby): %s (err=%v)\n", tag, err)

	// 5. Ensure Discord exists if shopify row didn't exist
	tag, err = pool.Exec(ctx,
		`INSERT INTO companies (name, slug, website, ats_platform, careers_url, industry)
		 VALUES ('Discord', 'discord', 'https://discord.com', 'greenhouse', 'https://discord.com/careers', 'Social / Communication')
		 ON CONFLICT (slug) DO NOTHING`)
	fmt.Printf("[5] Ensure Discord exists: %s (err=%v)\n", tag, err)

	// Show updated state
	fmt.Println("\n=== AFTER ===")
	rows, _ = pool.Query(ctx, "SELECT slug, ats_platform, name FROM companies ORDER BY name")
	for rows.Next() {
		var slug, ats, name string
		rows.Scan(&slug, &ats, &name)
		fmt.Printf("  %-20s %-12s %s\n", slug, ats, name)
	}
	rows.Close()

	fmt.Println("\n✅ Done! Companies are now pointing to the correct ATS platforms.")
}
