# NixOS pull request deduplication
Basic tool to find pullrequests in NixPkgs that might be able to be closed.
Basically run `./getprs.sh`, which will fetch all PRs from NixPkgs into `outputs/`, then just run `go run prdubs.go`.
You can edit the `nixpkgsPath` at the top of the file, to use the latest NixPkgs (local path).

It will output a list, which basically have the following criteria.

1. If there are two PRs for the same package show them
2. If the local NixPkgs version might be newer than the PR, show it
