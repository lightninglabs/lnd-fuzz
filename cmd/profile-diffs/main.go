package main

import (
	"flag"
	"fmt"
	"os"

	lndfuzz "github.com/lightninglabs/lnd-fuzz"
)

func main() {
	var (
		verbose bool
		summary bool
	)

	flag.BoolVar(&verbose, "v", false, "verbose output (list all new blocks)")
	flag.BoolVar(&summary, "summary", false, "only show summary, don't write output file")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <profile1_path> <profile2_path> <output_path>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nCompares two Go coverage profiles and outputs the new code blocks\n")
		fmt.Fprintf(os.Stderr, "that were hit in the second profile and not the first.\n\n")
		fmt.Fprintf(os.Stderr, "This can be used to determine if coverage has increased from the\n")
		fmt.Fprintf(os.Stderr, "first profile to the second profile.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExample:\n")
		fmt.Fprintf(os.Stderr, "  %s coverage/profile.old coverage/profile.new coverage/new_blocks.txt\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -summary coverage/profile.old coverage/profile.new -\n", os.Args[0])
	}

	flag.Parse()

	minArgs := 3
	if summary {
		minArgs = 2
	}

	if flag.NArg() < minArgs {
		flag.Usage()
		os.Exit(1)
	}

	firstProfilePath := flag.Arg(0)
	secondProfilePath := flag.Arg(1)
	var outputPath string
	if !summary {
		outputPath = flag.Arg(2)
	}

	// Create comparator and compare
	comparator := lndfuzz.NewProfileComparator()
	result, err := comparator.Compare(firstProfilePath, secondProfilePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error comparing profiles: %v\n", err)
		os.Exit(1)
	}

	// Print summary
	fmt.Printf("Comparison Results:\n")
	fmt.Printf("  Profile 1: %s\n", firstProfilePath)
	fmt.Printf("  Profile 2: %s\n", secondProfilePath)
	fmt.Printf("  Newly hit blocks: %d\n", result.TotalNewBlocks)
	fmt.Printf("  Total new hits: %d\n", result.TotalNewHits)

	if result.TotalNewBlocks == 0 {
		fmt.Println("\nNo new blocks were hit in the second profile.")
	} else if verbose {
		fmt.Println("\nNewly hit blocks:")
		for i, block := range result.NewlyHitBlocks {
			fmt.Printf("  %d. %s (hits: %d)\n", i+1, block.Block, block.HitCount)
			if i >= 10 && !verbose {
				fmt.Printf("  ... and %d more\n", result.TotalNewBlocks-10)
				break
			}
		}
	}

	// Write output if not summary mode
	if !summary {
		if err := lndfuzz.WriteComparisonResult(result, outputPath); err != nil {
			fmt.Fprintf(os.Stderr, "\nError writing output: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("\nNewly hit blocks have been saved to: %s\n", outputPath)
	}
}
