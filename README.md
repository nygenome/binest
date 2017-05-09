# binest

##### Description

binest calculates chromcopy, sex and normalized sizes per 16kb chunk in the genome
from the BAM index. 

In order to map the chunk values back to their genomic co-ordinates,
binest tries to read the BAM header for the corresponding BAM file.
If the BAM file doesn't exist, the reference FAI index must be provided.

Note: Any TABIX indexed file can be used with binest to get an idea of data density across the genome.
The reference FAI index must always be provided when working with TABIX indexes.


##### Example usage

The basic usage for all binest commands are the same.
Examples below show usage for the size command.

```shell
# Scenario 1 - BAM and BAI for sample present
binest size [PATH_TO_BAI_FILE]
binest size [PATH_TO_BAI_FILE1] [PATH_TO_BAI_FILE2]...
ls {PROJECT}/{SAMPLE}_*/*.bai | binest size

# Scenario 2 - BAI only present. BAM not present.
binest size --fai [REFERENCE.fasta.fai] [PATH_TO_BAI_FILE]
binest size -f [REFERENCE.fasta.fai] [PATH_TO_BAI_FILE1] [PATH_TO_BAI_FILE2]...
ls {PROJECT}/{SAMPLE}_*/*.bai | binest size -f [REFERENCE.fasta.fai]

# Scenario 3 - TBI index.
binest size --fai [REFERENCE.fasta.fai] [PATH_TO_TBI_FILE]
binest size -f [REFERENCE.fasta.fai] [PATH_TO_TBI_FILE1] [PATH_TO_TBI_FILE2]...
ls {PROJECT}/{SAMPLE}_*/*.tbi | binest size -f [REFERENCE.fasta.fai]

## Additional parameters can be seen by running
binest -h
```