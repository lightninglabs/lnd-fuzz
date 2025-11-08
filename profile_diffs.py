import sys

def find_newly_hit_blocks(first_profile_path, second_profile_path, output_path):
    """
    Given two paths to Go coverage profiles, determine which basic blocks are hit
    in the second coverage profile and not the first. This can be used to tell if
    coverage has increased from the first profile to the second profile.
    TODO: Compare the # of times that a basic block has been hit as well.

    Args:
        first_profile_path (str): Path to the first Go coverage profile.
        second_profile_path (str): Path to the second Go coverage profile.
        output_path (str): Path to save the new code blocks.
    """
    try:
        # Read the two profiles
        with open(first_profile_path, 'r') as prof1, open(second_profile_path, 'r') as prof2:
            prof1_lines = prof1.readlines()
            prof2_lines = prof2.readlines()

        # Parse lines into sets of code blocks
        def parse_coverage(lines):
            coverage = {}
            for line in lines:
                # Split on whitespace
                parts = line.split()

                # The last digit in a coverage profile denotes how many times it was
                # hit during the profile. Extract this value. The "block" is the part
                # of the coverage profile _before_ the hit counter.
                if len(parts) > 0 and parts[-1].isdigit():
                    block = " ".join(parts[:-1])
                    hit = int(parts[-1])
                    coverage[block] = hit

            # Return the coverage map for comparison.
            return coverage

        file1_coverage = parse_coverage(prof1_lines)
        file2_coverage = parse_coverage(prof2_lines)

        # Find blocks hit in the second profile but not in the first
        newly_hit_blocks = []
        for block, hit in file2_coverage.items():
            if hit > 0 and file1_coverage.get(block, 0) == 0:
                newly_hit_blocks.append(f"{block} {hit}\n")

        # Write newly hit blocks to the output file
        with open(output_path, 'w') as output_file:
            output_file.writelines(newly_hit_blocks)

        print(f"Newly hit blocks have been extracted and saved to: {output_path}")

    except Exception as e:
        print(f"An error occurred: {e}")

def main(file1_path, file2_path, output_path):
    find_newly_hit_blocks(file1_path, file2_path, output_path)

if __name__ == "__main__":
    if len(sys.argv) != 4:
        print("Usage: python profile_diffs.py <profile1_path> <profile2_path> <output_path>")
        sys.exit(1)

    file1_path = sys.argv[1]
    file2_path = sys.argv[2]
    output_path = sys.argv[3]

    main(file1_path, file2_path, output_path)
