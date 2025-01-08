# goapp - MiTo Team's Golang projects application base

[![Reference](https://pkg.go.dev/badge/github.com/mitoteam/goapp.svg)](https://pkg.go.dev/github.com/mitoteam/goapp)
![GitHub code size](https://img.shields.io/github/languages/code-size/mitoteam/goapp)
[![Go Report Card](https://goreportcard.com/badge/github.com/mitoteam/goapp)](https://goreportcard.com/report/github.com/mitoteam/goapp)
![GitHub](https://img.shields.io/github/license/mitoteam/goapp)


[![GitHub Version](https://img.shields.io/github/v/release/mitoteam/goapp?logo=github)](https://github.com/mitoteam/goapp)
[![GitHub Release](https://img.shields.io/github/release-date/mitoteam/goapp)](https://github.com/mitoteam/goapp/releases)
[![GitHub commit activity](https://img.shields.io/github/commit-activity/y/mitoteam/goapp)](https://github.com/mitoteam/dhtml/commits)

Go projects application base

## Add as git submodule
```
git submodule add -b main https://github.com/mitoteam/goapp.git internal/goapp
```

Disable commit hash tracking for submodule in `.gitmodules` :
```
[submodule "goapp"]
    ignore = all # ignore hash changes
```

Add `use ./internal/goapp` to main project `go.work`

Do `go mod tidy`

## Useful commands

Pull main project with submodules:

```
git pull --recurse-submodules
```
