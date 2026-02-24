## [1.2.1](https://github.com/cesar32az/tfkill/compare/v1.2.0...v1.2.1) (2026-02-24)


### Bug Fixes

* **ui:** keep cursor visible on already-deleted rows ([b271e2d](https://github.com/cesar32az/tfkill/commit/b271e2d9e6e3a8d7963d4ff8226409dce9856de8))

# [1.2.0](https://github.com/cesar32az/tfkill/compare/v1.1.0...v1.2.0) (2026-02-24)


### Features

* **ui:** add dot leader between path and size columns ([71e33b9](https://github.com/cesar32az/tfkill/commit/71e33b96ccf22d2808b43fff097df65100349f41))
* **ui:** responsive layout, adaptive sizes, and styled key chips ([4b9f633](https://github.com/cesar32az/tfkill/commit/4b9f6334982dba6708069814c79adfe5ca957abd)), closes [#2](https://github.com/cesar32az/tfkill/issues/2) [#3](https://github.com/cesar32az/tfkill/issues/3) [#5](https://github.com/cesar32az/tfkill/issues/5) [#383838](https://github.com/cesar32az/tfkill/issues/383838) [#6](https://github.com/cesar32az/tfkill/issues/6)

# [1.1.0](https://github.com/cesar32az/tfkill/compare/v1.0.0...v1.1.0) (2026-02-24)


### Bug Fixes

* **cmd:** move --dry-run flag registration from Execute() to init() ([d4a6f46](https://github.com/cesar32az/tfkill/commit/d4a6f46461e72e5e133793a9f618e1a1ed3eed05))
* **scanner:** resolve race condition, surface walk errors, remove UI state ([c3ff871](https://github.com/cesar32az/tfkill/commit/c3ff87173a6a5ba5a3602282104112bffa4d9fa2))
* **ui:** surface delete errors and decouple deletion state from scanner ([42f1eb6](https://github.com/cesar32az/tfkill/commit/42f1eb6c7a9df74d7145ab433611f7c8b5ca8576))


### Features

* **scanner:** add worker pool and context cancellation support ([713b7c9](https://github.com/cesar32az/tfkill/commit/713b7c9b06af6d33b0b96ac5ae5dcbed555162f7))

# 1.0.0 (2026-02-24)


### Bug Fixes

* **ci:** commit package-lock.json so npm ci can run in GitHub Actions ([09ec7a5](https://github.com/cesar32az/tfkill/commit/09ec7a595f14f812513d0815f081b153b72d6a2e))


### Features

* **cli:** add --dry-run mode and optional path argument ([a419d6a](https://github.com/cesar32az/tfkill/commit/a419d6acc84112d1df7c084272e18914d58cf74f))
* **cmd:** add root CLI to launch interactive UI ([cdffd23](https://github.com/cesar32az/tfkill/commit/cdffd236da9c8c4b9fed0013cbf9c6c8e70b11a9))
* **golang-pro:** add Go pro skill references and templates ([1499104](https://github.com/cesar32az/tfkill/commit/1499104c2009590641a45202ed9ceae756ccaed6))
* **scanner:** adds detection of terraform cache dirs ([cddfef4](https://github.com/cesar32az/tfkill/commit/cddfef42cfe38e6d9e7d17d401a5fb25248b2572))
* **ui:** add interactive TUI to scan and delete .terraform folders ([48992a2](https://github.com/cesar32az/tfkill/commit/48992a2bc58d53591fcee8191cdd84c4b07175c7))
