# binest
> Fast estimates of copy number and sex using BAM index

##### USAGE
```shell
usage: binest [<flags>] <command> [<args> ...]

Estimate copy number, size and sex from BAI/TBI index bins.

Flags:
  -h, --help     Show context-sensitive help (also try --help-long and --help-man).
  -v, --version  Show application version.
  -f, --fai=FAI  path to reference FAI index.
  -c, --cores=1  number of cores to use.

Commands:
  help [<command>...]
    Show help.

  copy [<flags>] [<index>...]
    Estimate per chromosome copy number from one or more indexes (stdin or arguments).

  size [<flags>] [<index>...]
    Compute size across 16kb bins from one or more indexes (stdin or arguments).

  sex [<flags>] [<index>...]
    Estimate sex genotype of a sample from one or more indexes (stdin or arguments).
```