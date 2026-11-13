# Changelog

## [0.2.1](https://github.com/notzenco/wordle-solver/compare/v0.2.0...v0.2.1) (2026-11-13)


### Bug Fixes

* **clean:** delete settings too and name the kept eval transcripts ([7a01743](https://github.com/notzenco/wordle-solver/commit/7a0174321c5377c13ce0d910e9e91da3565781e0))
* **cli:** redact API keys in model replies shown on screen ([2528acd](https://github.com/notzenco/wordle-solver/commit/2528acd07a1b89253d9fecc1885f15999e7b157b))
* **cli:** turn on colour handling in Windows consoles ([6b7d171](https://github.com/notzenco/wordle-solver/commit/6b7d1712de8213d30cf6456374516e263a3e8f1a))
* **player:** merge overlapping saves under a lock and keep files private ([f16ba9c](https://github.com/notzenco/wordle-solver/commit/f16ba9c27a669dce99e2c35c4252c4cc6eee61d4))
* reject answers and saved words that aren't five letters a-z ([7f1870e](https://github.com/notzenco/wordle-solver/commit/7f1870e70fefad599c68331beac96a9223a451b7))
* **selfupdate:** cap the unpacked binary and write it to a unique file ([d9b57b3](https://github.com/notzenco/wordle-solver/commit/d9b57b39c8d9101d0ef3011c664366059b69e50c))
* **settings:** validate the approach and let settings repair bad values ([df15fd5](https://github.com/notzenco/wordle-solver/commit/df15fd5847804a2bb4ffd2955f8d5b3d4b8a3b99))

## [0.2.0](https://github.com/notzenco/wordle-solver/compare/v0.1.0...v0.2.0) (2026-11-13)


### ⚠ BREAKING CHANGES

* **cli:** play no longer suggests guesses; use assist --manual for the previous behaviour. play --model is replaced by assist --by model --model, and the play.model setting by model.
* **cli:** options that do not belong to a command are now rejected instead of ignored (for example `play --strategy`). Eval writes transcripts to the data folder unless --out is given. The providers and settings listings gained header rows, so scripts parsing them must skip the first line.

### Features

* **auto:** reject invalid model words without using a turn ([0ee04a1](https://github.com/notzenco/wordle-solver/commit/0ee04a1f976b8369ff9cd8fec263371a6f1e86ea))
* **cli:** add play, assist and auto approaches ([12b1ee6](https://github.com/notzenco/wordle-solver/commit/12b1ee65d4c47a169f14bb36e96d14f5a7f356a3))
* **cli:** give each command its own options and help ([5fe2023](https://github.com/notzenco/wordle-solver/commit/5fe2023e64fb16be597a35c50470992490ec0175))
* **eval:** score model games against entropy with transcripts ([f2a52f6](https://github.com/notzenco/wordle-solver/commit/f2a52f6bf380921a51b664c7c9053a8c53d7509a))
* **eval:** support systemone decisions and candidate scoring ([6653136](https://github.com/notzenco/wordle-solver/commit/66531364f5aeb3c8499a32022d61e89aa41dfd96))
* **play:** take suggestions from a model with --model ([d76cbd8](https://github.com/notzenco/wordle-solver/commit/d76cbd8d0ffdc1c5f71abd60530f14fac4f9be6b))
* **providers:** configure compatible servers and list models ([351d25e](https://github.com/notzenco/wordle-solver/commit/351d25ea4f08e1d2b99585751854aa31a6cbb388))
* **release:** ship wsolve as a short name for wordle-solver ([e124bec](https://github.com/notzenco/wordle-solver/commit/e124bec78c174e189fe8efde2557f5ab439da7f3))
* **settings:** persist command defaults with source precedence ([2d17b71](https://github.com/notzenco/wordle-solver/commit/2d17b7190c9edb84c79bcaa59c3f819570eb59f5))
* **wordle:** suggest the next guess for any game in progress ([209972f](https://github.com/notzenco/wordle-solver/commit/209972fb46d091eeda318f8d31da6866398e9cd2))


### Bug Fixes

* **config:** validate saved providers and redact echoed keys ([3a75df6](https://github.com/notzenco/wordle-solver/commit/3a75df6068a9c0495261b34b89f53cc2cba78f41))
* **eval:** accept guesses inside a markdown code fence ([308a882](https://github.com/notzenco/wordle-solver/commit/308a8826725cf0dd16636276ce257aa1cf14e85a))
* **eval:** format terminal summaries for readable results ([71810f2](https://github.com/notzenco/wordle-solver/commit/71810f2d16caf2a139d1c711d87424a845632d92))
* **eval:** weight the frequency baseline by familiarity ([41dbf17](https://github.com/notzenco/wordle-solver/commit/41dbf17504b2ff487ba6486491c68b40cad5c520))

## 0.1.0 (2026-11-13)


### ⚠ BREAKING CHANGES

* `wordle-solver schedule`, `auto -catchup` and `-log` no longer exist. Remove an installed daily run before upgrading: on macOS, `launchctl bootout gui/$(id -u)/wordle-solver.daily` and delete ~/Library/LaunchAgents/wordle-solver.daily.plist; on Linux, `systemctl --user disable --now wordle-solver.timer`; on Windows, `schtasks /Delete /TN "wordle-solver daily" /F`.

### Features

* add human-looking wordle solver ([#1](https://github.com/notzenco/wordle-solver/issues/1)) ([fa54f16](https://github.com/notzenco/wordle-solver/commit/fa54f16ae83c96ebc18a30008e65a51e49a999c4))
* add releases, self-update, clean and cross-platform schedule ([#3](https://github.com/notzenco/wordle-solver/issues/3)) ([97df70a](https://github.com/notzenco/wordle-solver/commit/97df70a068b2e1546eef7de8ad109d3b27d3e4dc))
* **bench:** compare five guess strategies over every answer ([#5](https://github.com/notzenco/wordle-solver/issues/5)) ([b631f87](https://github.com/notzenco/wordle-solver/commit/b631f875a8879d6284fdf5a5c44ef8ad30cb370c))
* play as one consistent, more human player ([#2](https://github.com/notzenco/wordle-solver/issues/2)) ([975ba95](https://github.com/notzenco/wordle-solver/commit/975ba9534641f0930b7c55374a81a52e60a6cfb3))
* remove record shaping and the schedule command ([#4](https://github.com/notzenco/wordle-solver/issues/4)) ([cad386e](https://github.com/notzenco/wordle-solver/commit/cad386e543c7bddb6e6bead82e82ac34af4d7ce3))


### Documentation

* rewrite readme around the solver and its benchmark ([#7](https://github.com/notzenco/wordle-solver/issues/7)) ([6d0bae0](https://github.com/notzenco/wordle-solver/commit/6d0bae069df7f6de26806394c7c7fb8fec612c07))
