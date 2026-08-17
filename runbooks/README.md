# Runbooks

Step-by-step for the things that aren't obvious from the code.

| runbook | when |
| --- | --- |
| [release.md](release.md) | cutting a new version |
| [website.md](website.md) | changing or redeploying the site |
| [screenshot.md](screenshot.md) | regenerating the terminal screenshot |
| [develop.md](develop.md) | building, testing, and finding your way around |

Each one lists what to run, how to check it worked, and how to undo it.

## The short version

```sh
make check                          # fmt, vet, test — before anything else
git tag -a v0.3.0 -m "tuki v0.3.0"  # cut a release
git push origin v0.3.0              # CI builds and publishes it
git push origin main                # deploy the site (Pages serves /docs)
```
