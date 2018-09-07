<a name="unreleased"></a>
## [Unreleased]


<a name="v0.8.4"></a>
## [v0.8.4] - 2018-09-07
### Bug Fixes
- remove -s to prevent fail on gpg sign failure
- typo in makefile

### Code Maintenance
- Bump version to v0.8.3
- add current version file
- use git-chglog full binary path
- disable gosec linter
- update golangci-lint version

### New Features
- add XO -> XX case, when xNorm > 1.5 and yNorm < 0.25, break func


<a name="v0.8.2"></a>
## [v0.8.2] - 2018-08-23
### Bug Fixes
- add case for XOs with lower than 0.25 yNorm

### Code Maintenance
- Bump version to v0.8.2


<a name="v0.8.1"></a>
## [v0.8.1] - 2018-08-23
### Code Maintenance
- Bump version to v0.8.1
- fix ineffectual assignment


<a name="v0.8.0"></a>
## [v0.8.0] - 2018-08-23
### Code Maintenance
- Bump version to v0.8.0
- remove unused functions
- remove dep ensure task from deploy

### New Features
- call XO/XY mosaics male when yNorm is between 0.25 and 0.7


<a name="v0.7.0"></a>
## [v0.7.0] - 2018-07-31
### Code Maintenance
- Bump version to v0.7.0
- comment out unused functions
- Bump version to v0.7.0

### New Features
- use autosome byte size median and within chromosome median to compute chromosome copy estimate
- use only autosomes to get median byte size for per bin normalization


<a name="v0.6.2"></a>
## [v0.6.2] - 2018-07-30
### Bug Fixes
- strip known suffixes to try to get sample name, also maintains compatibility with older versions

### Code Maintenance
- Bump version to v0.6.2


<a name="v0.6.1"></a>
## [v0.6.1] - 2018-07-30
### Bug Fixes
- use same header as previous versions to maintain as much compatibility as possible
- remove dep ensure from all target

### Code Maintenance
- Bump version to v0.6.1
- fix changes due to dep v0.4 and v0.5 clash. use v0.5
- fix changes to dep lock. pin to dep v0.5.x


<a name="v0.6.0"></a>
## [v0.6.0] - 2018-07-30
### Bug Fixes
- fallback to old normalize method

### Code Maintenance
- Bump version to v0.6.0
- simplify tasks, add default to all
- fix changes due to dep v0.4 and v0.5 clash. use v0.5
- remove extra newline
- update dep lock file
- parse command before using, remove always print version statement
- update dep lock file

### New Features
- use multi step normalize, one within chromosome and one across chromosomes
- skip and ignore bins which are always zero
- add zero refbins resource file and update deps
- detect reference build from refmap and use in creating index
- add tool to build zero bins resource file


<a name="v0.5.0"></a>
## [v0.5.0] - 2018-07-27
### Bug Fixes
- print XO and OY when partially present
- preallocate the right slice length and append

### Code Maintenance
- Bump version to v0.5.0
- write build target in bin dir
- update to dep version v0.5.0
- fix to set package name

### New Features
- major refactor to make adding/testing new features easier and some perf improvements


<a name="v0.4.4"></a>
## [v0.4.4] - 2018-07-25
### Code Maintenance
- Bump version to v0.4.4
- fix GOOS to darwin


<a name="v0.4.3"></a>
## v0.4.3 - 2018-07-25
### Bug Fixes
- update import paths

### Code Maintenance
- Bump version to v0.4.3
- fix lint issues and update deps
- add golangci-lint config
- ignore binaries, vendor pkgs and restart git history to fix large repo size

### New Features
- add git changelog config
- add deploybot tool
- add auto versioning, deploy Makefile


[Unreleased]: https://git.nygenome.org/rmusunuri/binest/compare/v0.8.4...HEAD
[v0.8.4]: https://git.nygenome.org/rmusunuri/binest/compare/v0.8.2...v0.8.4
[v0.8.2]: https://git.nygenome.org/rmusunuri/binest/compare/v0.8.1...v0.8.2
[v0.8.1]: https://git.nygenome.org/rmusunuri/binest/compare/v0.8.0...v0.8.1
[v0.8.0]: https://git.nygenome.org/rmusunuri/binest/compare/v0.7.0...v0.8.0
[v0.7.0]: https://git.nygenome.org/rmusunuri/binest/compare/v0.6.2...v0.7.0
[v0.6.2]: https://git.nygenome.org/rmusunuri/binest/compare/v0.6.1...v0.6.2
[v0.6.1]: https://git.nygenome.org/rmusunuri/binest/compare/v0.6.0...v0.6.1
[v0.6.0]: https://git.nygenome.org/rmusunuri/binest/compare/v0.5.0...v0.6.0
[v0.5.0]: https://git.nygenome.org/rmusunuri/binest/compare/v0.4.4...v0.5.0
[v0.4.4]: https://git.nygenome.org/rmusunuri/binest/compare/v0.4.3...v0.4.4
