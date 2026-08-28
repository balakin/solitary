# Changelog

## [0.5.2](https://github.com/balakin/solitary/compare/v0.5.1...v0.5.2) (2026-08-28)


### Bug Fixes

* correct a description that says the opposite of what a cell does ([18825b0](https://github.com/balakin/solitary/commit/18825b0423f59fe14aa0d677843efbcfbed5e351))

## [0.5.1](https://github.com/balakin/solitary/compare/v0.5.0...v0.5.1) (2026-08-28)


### Bug Fixes

* refuse an IPv6 address a cell cannot reach ([78d515b](https://github.com/balakin/solitary/commit/78d515b7fd1b58c3798faf7bdeda9acaf8becb7c))
* refuse an IPv6 address a cell cannot reach ([2c07573](https://github.com/balakin/solitary/commit/2c075731f08a8cbbc64a4932b19b517536665c0b))

## [0.5.0](https://github.com/balakin/solitary/compare/v0.4.0...v0.5.0) (2026-08-28)


### ⚠ BREAKING CHANGES

* let a cell say which secrets it can do without ([#34](https://github.com/balakin/solitary/issues/34))
* secrets: is a mapping of name to options, not a list of names. Drop the dashes: "- GITHUB_TOKEN" becomes "GITHUB_TOKEN:". A cell still using the list form refuses to load and names the line to write.

### Features

* let a cell say which secrets it can do without ([da6f58d](https://github.com/balakin/solitary/commit/da6f58dea9a494410c3f9ff2efba5e6c171525a5))
* let a cell say which secrets it can do without ([#34](https://github.com/balakin/solitary/issues/34)) ([395ac30](https://github.com/balakin/solitary/commit/395ac30587e689ede2d8645694bab7201508c260))

## [0.4.0](https://github.com/balakin/solitary/compare/v0.3.0...v0.4.0) (2026-08-26)


### Features

* add example cells for claude and vscode ([d2118cc](https://github.com/balakin/solitary/commit/d2118cc5790be52950adef95f23d6623fdb6069c))
* add example cells for claude and vscode ([#31](https://github.com/balakin/solitary/issues/31)) ([824dd07](https://github.com/balakin/solitary/commit/824dd0727abd1c1b310414ccd918348600790db3))
* give the vscode example a theme, claude's extension and the usual ports ([5757883](https://github.com/balakin/solitary/commit/5757883c8dac764be1651d835f764cc67bf03ccf))


### Bug Fixes

* make the example cells' first start and build exit cleanly ([18da387](https://github.com/balakin/solitary/commit/18da3875be0bf6b98bcc8caac406bc50371c9d23))
* make the vscode example show an extension a rebuilt image added ([c95a4fa](https://github.com/balakin/solitary/commit/c95a4fab33c18a020b16aa24578469f039d8941b))
* refuse a vm.disk smaller than the machine already has ([7d7cd5d](https://github.com/balakin/solitary/commit/7d7cd5d310357afc46d4c783915a4b702cb245b2))
* refuse a vm.disk smaller than the machine already has ([#32](https://github.com/balakin/solitary/issues/32)) ([b518b6e](https://github.com/balakin/solitary/commit/b518b6ed01af1c54c10c3cceddfe5d8329a91f94))
* set the vscode example's editor defaults where the editor reads them ([95855f0](https://github.com/balakin/solitary/commit/95855f09cb9af61727d8434e27783c82e69470d8))

## [0.3.0](https://github.com/balakin/solitary/compare/v0.2.0...v0.3.0) (2026-08-19)


### Features

* add solitary doctor for host-level checks ([6fbfb15](https://github.com/balakin/solitary/commit/6fbfb15e322dd95d713b9708464c742ec24e4ac7))


### Bug Fixes

* write the Homebrew formula to Formula/ ([cd6b472](https://github.com/balakin/solitary/commit/cd6b4723c74a279ecf4ef7b8824496385b859769))

## [0.2.0](https://github.com/balakin/solitary/compare/v0.1.1...v0.2.0) (2026-08-19)


### Features

* add an update command and a daily release check ([379f181](https://github.com/balakin/solitary/commit/379f181006fbe1d1ba7fae48f11744b206f20cb0))
* install and update solitary without the release page ([e56c323](https://github.com/balakin/solitary/commit/e56c323d900461df9fa3e268b4f6eddd6b47b541))
* publish a homebrew formula on release ([f37c80b](https://github.com/balakin/solitary/commit/f37c80b42c9ed26b0286f382098a7fdcf077375a))
* **website:** serve the site from solitary.balakin.io ([619fd0f](https://github.com/balakin/solitary/commit/619fd0f24f41dabb05191026949e194feb5e6ee7))
* **website:** serve the site from solitary.balakin.io ([4b9f600](https://github.com/balakin/solitary/commit/4b9f600238531ca36f2ba2ceecefa479ebab13d4))


### Bug Fixes

* **website:** stop asking a static host for the route manifest ([3186e50](https://github.com/balakin/solitary/commit/3186e50a0103961ec8a35b91f75aa3b8fdd3d133))

## [0.1.1](https://github.com/balakin/solitary/compare/v0.1.0...v0.1.1) (2026-08-18)


### Features

* apply vm changes when a stopped cell starts ([14d1d68](https://github.com/balakin/solitary/commit/14d1d683294f6ae78c7979db9fe251af01ffa157))
* build a cell's image from a Containerfile ([6e8cfb5](https://github.com/balakin/solitary/commit/6e8cfb544880294eea24a4304deeb305339467cd))
* create, start, stop and destroy cell machines ([45c2ed4](https://github.com/balakin/solitary/commit/45c2ed4a9e77737a6cc63f9af5e4b108206d8366))
* fetch what a cell published, and send files into it ([0a6cb1c](https://github.com/balakin/solitary/commit/0a6cb1cb3612787baef45ca39c666812767050cb))
* follow a cell's network as it happens ([e66fa3f](https://github.com/balakin/solitary/commit/e66fa3fec1158c7a76a07725adf2eacd6e53a61e))
* give a cell an outbox and an inbox ([6bb63e4](https://github.com/balakin/solitary/commit/6bb63e409235613fa59a39b18ce885d3b1310667))
* give cells a git identity ([a133e72](https://github.com/balakin/solitary/commit/a133e720f8ff221625c4ab5da18aae0176c341e9))
* hand out the command that attaches to a cell, and describe one ([90e2d7f](https://github.com/balakin/solitary/commit/90e2d7fc3c375e086e51bf573c1347a0abf5511c))
* hold a cell to an egress allow list ([ef102b3](https://github.com/balakin/solitary/commit/ef102b3b395ba15f5e844db84eb1d26b2bb65f23))
* let a cell name the resolvers it forwards to ([85d61bd](https://github.com/balakin/solitary/commit/85d61bd936583558e4bb87b5b5ca29fedd81314f))
* manage cells in a dashboard ([1522a76](https://github.com/balakin/solitary/commit/1522a76f720271759ececc6ea56d553319377496))
* pass whitelisted secrets into cells ([ef8833c](https://github.com/balakin/solitary/commit/ef8833c38663fb6170593db5f9ac7ff13221d1a7))
* read a cell's traffic, not just watch it go past ([3aa3200](https://github.com/balakin/solitary/commit/3aa32001585f59c95f8e5f442a2bcccc0b85a31b))
* resolve cell config and render Lima definitions ([75b4e8a](https://github.com/balakin/solitary/commit/75b4e8ab3d025b0a36bddbaff9d818c3ac1aafa4))
* run a single command in a cell ([c612d7d](https://github.com/balakin/solitary/commit/c612d7d50709c5220bff13ea471c3d2aae6ec1b4))
* run the toolset in a container inside the cell ([dc0013e](https://github.com/balakin/solitary/commit/dc0013ec15e34a301b0633babcb9ab35cb296417))
* say when a tunnelled cell still resolves outside its tunnel ([5b59bb0](https://github.com/balakin/solitary/commit/5b59bb0421183057b8edab66dc6370b408cd56d4))
* send a cell's traffic through a wireguard tunnel ([b26290d](https://github.com/balakin/solitary/commit/b26290d6db7b6d233624e77da51e5c38db110e51))
* show a cell's tunnel in the dashboard ([f0b12f5](https://github.com/balakin/solitary/commit/f0b12f5dcf2f73b02566aebbcb6b45661f28585c))
* show what a cell may reach in the dashboard ([4f05ca9](https://github.com/balakin/solitary/commit/4f05ca9363122c2d2cd1d503447435f8c8945532))
* show what a cell's tunnel is doing in the dashboard ([cc60bd7](https://github.com/balakin/solitary/commit/cc60bd74c8e617a448d59ab5ccc514b757ab06cb))
* take one cell out of someone's repository ([ec2191a](https://github.com/balakin/solitary/commit/ec2191a3cbd979ea7dcc5ffe6c0ede18388e0ed5))
* take one cell out of someone's repository ([1c95bc3](https://github.com/balakin/solitary/commit/1c95bc3247bb6bad6832cece9ff5c8cc11bc7e74))
* tell a cell its own name ([407429d](https://github.com/balakin/solitary/commit/407429d71d92b1bd29c10dec6f61bacc8c9df339))


### Bug Fixes

* carry the terminal into a cell ([3d8d2a8](https://github.com/balakin/solitary/commit/3d8d2a8cd760bd5d89cb8c69df65c361da44e2d0))
* deny the ports a cell did not allow ([581e425](https://github.com/balakin/solitary/commit/581e4255a49a48c029ba2641bf68a510701b4649))
* keep re-reading a cell's tunnel while it is on screen ([78fb50f](https://github.com/balakin/solitary/commit/78fb50f1e50c4e61994a97386f89f7fe0b16e1df))
* make a change to a cell reach its machine, both ways ([dc6c364](https://github.com/balakin/solitary/commit/dc6c3643b51fe47944d1073df360e01ce15317af))
* only check host memory when a machine is about to take it ([b40072d](https://github.com/balakin/solitary/commit/b40072d33670a105a995383af0a879d5124cffaa))
* refuse machines the host cannot back, and notice dead ones ([0789100](https://github.com/balakin/solitary/commit/078910016b2f88e5a672452e0e7fa0f8de1be85b))


### Refactors

* drop the compatibility path for the old definition file ([3bbd460](https://github.com/balakin/solitary/commit/3bbd4608c8f43b666a18919badb169a60153168f))
* keep the rendered machine definition out of the cell directory ([8ef6c75](https://github.com/balakin/solitary/commit/8ef6c756d08b65ec3d2e48f02ec9cd30131cad69))

## 0.1.0 (2026-08-18)


### Features

* apply vm changes when a stopped cell starts ([14d1d68](https://github.com/balakin/solitary/commit/14d1d683294f6ae78c7979db9fe251af01ffa157))
* build a cell's image from a Containerfile ([6e8cfb5](https://github.com/balakin/solitary/commit/6e8cfb544880294eea24a4304deeb305339467cd))
* create, start, stop and destroy cell machines ([45c2ed4](https://github.com/balakin/solitary/commit/45c2ed4a9e77737a6cc63f9af5e4b108206d8366))
* fetch what a cell published, and send files into it ([0a6cb1c](https://github.com/balakin/solitary/commit/0a6cb1cb3612787baef45ca39c666812767050cb))
* follow a cell's network as it happens ([e66fa3f](https://github.com/balakin/solitary/commit/e66fa3fec1158c7a76a07725adf2eacd6e53a61e))
* give a cell an outbox and an inbox ([6bb63e4](https://github.com/balakin/solitary/commit/6bb63e409235613fa59a39b18ce885d3b1310667))
* give cells a git identity ([a133e72](https://github.com/balakin/solitary/commit/a133e720f8ff221625c4ab5da18aae0176c341e9))
* hand out the command that attaches to a cell, and describe one ([90e2d7f](https://github.com/balakin/solitary/commit/90e2d7fc3c375e086e51bf573c1347a0abf5511c))
* hold a cell to an egress allow list ([ef102b3](https://github.com/balakin/solitary/commit/ef102b3b395ba15f5e844db84eb1d26b2bb65f23))
* let a cell name the resolvers it forwards to ([85d61bd](https://github.com/balakin/solitary/commit/85d61bd936583558e4bb87b5b5ca29fedd81314f))
* manage cells in a dashboard ([1522a76](https://github.com/balakin/solitary/commit/1522a76f720271759ececc6ea56d553319377496))
* pass whitelisted secrets into cells ([ef8833c](https://github.com/balakin/solitary/commit/ef8833c38663fb6170593db5f9ac7ff13221d1a7))
* read a cell's traffic, not just watch it go past ([3aa3200](https://github.com/balakin/solitary/commit/3aa32001585f59c95f8e5f442a2bcccc0b85a31b))
* resolve cell config and render Lima definitions ([75b4e8a](https://github.com/balakin/solitary/commit/75b4e8ab3d025b0a36bddbaff9d818c3ac1aafa4))
* run a single command in a cell ([c612d7d](https://github.com/balakin/solitary/commit/c612d7d50709c5220bff13ea471c3d2aae6ec1b4))
* run the toolset in a container inside the cell ([dc0013e](https://github.com/balakin/solitary/commit/dc0013ec15e34a301b0633babcb9ab35cb296417))
* say when a tunnelled cell still resolves outside its tunnel ([5b59bb0](https://github.com/balakin/solitary/commit/5b59bb0421183057b8edab66dc6370b408cd56d4))
* send a cell's traffic through a wireguard tunnel ([b26290d](https://github.com/balakin/solitary/commit/b26290d6db7b6d233624e77da51e5c38db110e51))
* show a cell's tunnel in the dashboard ([f0b12f5](https://github.com/balakin/solitary/commit/f0b12f5dcf2f73b02566aebbcb6b45661f28585c))
* show what a cell may reach in the dashboard ([4f05ca9](https://github.com/balakin/solitary/commit/4f05ca9363122c2d2cd1d503447435f8c8945532))
* show what a cell's tunnel is doing in the dashboard ([cc60bd7](https://github.com/balakin/solitary/commit/cc60bd74c8e617a448d59ab5ccc514b757ab06cb))
* take one cell out of someone's repository ([ec2191a](https://github.com/balakin/solitary/commit/ec2191a3cbd979ea7dcc5ffe6c0ede18388e0ed5))
* take one cell out of someone's repository ([1c95bc3](https://github.com/balakin/solitary/commit/1c95bc3247bb6bad6832cece9ff5c8cc11bc7e74))
* tell a cell its own name ([407429d](https://github.com/balakin/solitary/commit/407429d71d92b1bd29c10dec6f61bacc8c9df339))


### Bug Fixes

* carry the terminal into a cell ([3d8d2a8](https://github.com/balakin/solitary/commit/3d8d2a8cd760bd5d89cb8c69df65c361da44e2d0))
* deny the ports a cell did not allow ([581e425](https://github.com/balakin/solitary/commit/581e4255a49a48c029ba2641bf68a510701b4649))
* keep re-reading a cell's tunnel while it is on screen ([78fb50f](https://github.com/balakin/solitary/commit/78fb50f1e50c4e61994a97386f89f7fe0b16e1df))
* make a change to a cell reach its machine, both ways ([dc6c364](https://github.com/balakin/solitary/commit/dc6c3643b51fe47944d1073df360e01ce15317af))
* only check host memory when a machine is about to take it ([b40072d](https://github.com/balakin/solitary/commit/b40072d33670a105a995383af0a879d5124cffaa))
* refuse machines the host cannot back, and notice dead ones ([0789100](https://github.com/balakin/solitary/commit/078910016b2f88e5a672452e0e7fa0f8de1be85b))


### Refactors

* drop the compatibility path for the old definition file ([3bbd460](https://github.com/balakin/solitary/commit/3bbd4608c8f43b666a18919badb169a60153168f))
* keep the rendered machine definition out of the cell directory ([8ef6c75](https://github.com/balakin/solitary/commit/8ef6c756d08b65ec3d2e48f02ec9cd30131cad69))
