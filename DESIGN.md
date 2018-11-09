# How binest works

This is a design document which provides an overview of how binest works.
It's not intended to provide a complete description of all the technical details,
but will link to relevant external sources for those inclined to understand all the details.

 In its most basic form, binest uses the BAM index (`*.bai`) to quickly estimate the data density
 (in bytes) for every 16384 bp (~16 kb) window in the reference. Normalized coverage and sex
 estimates are made after normalizing the data density from the BAM index.
 
A brief primer on BGZF and BAM index formats would be helpful in understanding how binest works.

#### BGZF format
Any file compressed with the `bgzip` uses the BGZF format. It builds on top of the popular [GZIP](https://tools.ietf.org/html/rfc1952) compression format.
A BGZF file is a concatentation of BGZF blocks, where each BGZF block is just a GZIP file. 

The main reason for breaking the whole file into individual blocks is to enable efficient random access into the file.
Instead of decompressing the entire file on every read, BGZF enables decompressing just the blocks we need for the read operation.

[Section 4.1 in the SAM/BAM v1 specfication](https://samtools.github.io/hts-specs/SAMv1.pdf) describes the BGZF compression format.

[Biopython's bgzf module documentation](https://github.com/biopython/biopython/blob/biopython-172/Bio/bgzf.py#L39) provides a much more approachable introduction to the format.
 
#### BAM index
Apart from other things, the BAM index stores a linear index of the BAM.
The linear index stores the BGZF offset for every 16384 bp window on the reference.

This BGZF offset provides us two things:
* byte offset to first BGZF block that contains data for the 16kb interval.
* byte offset inside the decompressed BGZF block pointing to the first read in the 16kb interval. 

[Section 5 in the SAM/BAM v1 specification](https://samtools.github.io/hts-specs/SAMv1.pdf) describes the BAM index format. 

#### Normalization
Using the BGZF offset for every 16kb window, we can get the data density (in bytes) for each window.
The `binest size --raw` command would give exactly this, raw data density in each window of the reference. 

Because of the way [DEFLATE algorithm works](https://www.infinitepartitions.com/art001.html),
it is not a reliable indicator for the exact number of reads in the window.
But this works well as a good heuristic in most cases.

The median data density of all autosomal 16kb windows is used to normalize each window's size.
This is done under the assumption that this value would represent the data density in a 16kb region with normal ploidy.
In cases where the median value does not accurately represent a normal window, the normalized sizes
can be off.

The normalized size results from `binest size` are sizes of each window relative to this median value. These normalized
size values are also good estimates of normalized coverage for the window.
   