# Bloom tvOS app (Phase 0)

The sources in `Sources/` are plain SwiftUI. There is no `.xcodeproj` yet; make
one once and the files drop in. These steps are written from Apple's usual
Xcode flow and have **not been run yet**, so expect to adjust a menu name.

## One-time setup (on the MacBook Air)

1. Install Xcode (26.3 or newer if you want Claude Agent inside it).
2. In Xcode: **File > New > Project > tvOS > App**. Name it `BloomTV`, interface
   SwiftUI, and save it in this `apple/` folder.
3. Delete the template's `ContentView.swift` and `...App.swift`, then drag the
   files from `Sources/` in this repo into the project (tick "Copy items if
   needed" off, so they stay in the repo).
4. Sign in to your Apple ID under **Xcode > Settings > Accounts** (a free ID is
   enough for Phase 0) and set the project's Team under **Signing & Capabilities**.
5. Open `Config.swift` and set `serverURL` to your server, for example
   `http://192.168.1.177:8484`, and `token` if you set `BLOOM_TOKEN`.
6. Allow plain HTTP on your home network. In the target's **Info** tab add:
   - `App Transport Security Settings` > `Allow Local Networking` = `YES`
   - `Privacy - Local Network Usage Description` = "Bloom connects to your media server on your home network."

## Run on an Apple TV

1. Put the Apple TV and the Mac on the same network.
2. On the Apple TV: **Settings > Remotes and Devices > Remote App and Devices**.
3. In Xcode: **Window > Devices and Simulators > +**, pick the Apple TV, and
   enter the code shown on the TV.
4. Choose the Apple TV as the run destination and press Run.

Free Apple IDs expire the installed app after about seven days; reinstall from
Xcode. The paid program removes that limit.

## What to check

- Pick a title. Xcode's console prints `Bloom: playback started after N s`.
  Record that number; start-up time is the main Phase 0 measurement.
- Play a 4K HDR or Dolby Vision file and confirm the TV switches modes.
- Try a file with DTS or TrueHD audio and confirm you hear sound (it is converted).
