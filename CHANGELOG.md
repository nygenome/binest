<a name="unreleased"></a>
## [Unreleased]


<a name="v0.6.0"></a>
## [v0.6.0] - 2018-07-30
### Bug Fixes
- fallback to old normalize method

### Code Maintenance
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


[Unreleased]: https://git.nygenome.org/rmusunuri/binest/compare/v0.6.0...HEAD
[v0.6.0]: https://git.nygenome.org/rmusunuri/binest/compare/v0.5.0...v0.6.0
[v0.5.0]: https://git.nygenome.org/rmusunuri/binest/compare/v0.4.4...v0.5.0
[v0.4.4]: https://git.nygenome.org/rmusunuri/binest/compare/v0.4.3...v0.4.4
