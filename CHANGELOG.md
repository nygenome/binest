<a name="unreleased"></a>
## [Unreleased]


<a name="v0.5.0"></a>
## [v0.5.0] - 2018-07-27
### Bug Fixes
- print XO and OY when partially present
- preallocate the right slice length and append

### Code Maintenance
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


[Unreleased]: https://git.nygenome.org/rmusunuri/binest/compare/v0.5.0...HEAD
[v0.5.0]: https://git.nygenome.org/rmusunuri/binest/compare/v0.4.4...v0.5.0
[v0.4.4]: https://git.nygenome.org/rmusunuri/binest/compare/v0.4.3...v0.4.4
