# Changelog

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
