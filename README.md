
---

# Deej MacroPad

# <p align="center"> `This project is a work in progress` </p>
Deej MacroPad is a customizable open-source hardware volume mixer and macro pad designed for Windows ~~(and Linux)~~ PCs. Based on the original Deej project, this version adds additional functionality such as macro keys and a screen, creating a device similar to a StreamDeck. It lets you control the volumes of different apps and execute macros seamlessly using real-life sliders and buttons.

<!-- [![Discord](https://img.shields.io/discord/702940502038937667?logo=discord)](https://discord.gg/nf88NJu)

**[Download the latest release](https://github.com/omriharel/deej/releases/latest) | [Video demonstration](https://youtu.be/VoByJ4USMr8) | [Build video by Tech Always](https://youtu.be/x2yXbFiiAeI)** -->

<!-- ![Deej MacroPad](assets/build-3d-annotated.png) -->

<!-- > **_New:_** [work-in-progress Deej FAQ](./docs/faq/faq.md)! -->

## Table of Contents

- [Deej MacroPad](#deej-macropad)
- [ `This project is a work in progress` ](#-this-project-is-a-work-in-progress-)
  - [Table of Contents](#table-of-contents)
  - [About The Project](#about-the-project)
    - [Features](#features)
    - [Built With](#built-with)
    - [Hardware Components](#hardware-components)
  - [Getting Started](#getting-started)
    - [Prerequisites](#prerequisites)
    - [Installation](#installation)
  - [How It Works](#how-it-works)
    - [Hardware](#hardware)
      - [Schematic](#schematic)
    - [Software](#software)
  - [Slider Mapping (Configuration)](#slider-mapping-configuration)
      - [EXAMPLE](#example)
  - [Roadmap](#roadmap)
  - [Build Your Own](#build-your-own)
    - [Bill of Materials](#bill-of-materials)
  - [Contributing](#contributing)
  - [License](#license)
  - [Acknowledgments](#acknowledgments)

## About The Project

This project is derived from the Deej project by Omri Harel. It is an awesome project for controlling the audio of specific or multiple applications on Windows and Linux. This version enhances the original Deej by adding macro keys and a screen, offering extended functionality similar to a StreamDeck.\




### Features

Deej MacroPad is written in Go and is distributed as a portable (no installer needed) executable. It allows you to:

- Bind apps to different encoder
  - Bind multiple apps per encoder
  - Bind the master channel, microphone input level, and specific audio devices
  - Bind currently active app (on Windows)
- Control your microphone's input level
- Execute macros with additional buttons
- Display information on a connected screen
- Lightweight desktop client, consuming around 10MB of memory
- Runs from your system tray with helpful notifications

### Built With

- [![Go][Go.js]][Golang-url]
- [![EasyEDA][EasyEDA.js]][EasyEDA-url]
- [![WOKWI][wokwi.js]][Wokwi-url]
  - Andriod simulator : [Arduino uno](https://wokwi.com/projects/new/arduino-uno)
  - Diagram used for basic screen testing: [File](.\Readme_Assets\WOKWI_ArdionoUno_diagram.json) \
   <img src="Readme_Assets\Arduino simulator hardware setup.png" alt="Logo" width="200" height="200" >
<!-- - [![Arduino][Arduino.js]][Arduino-url] -->

### Hardware Components

All components for this hobby project were sourced from AliExpress:

- **Cherry-like keys:** [AliExpress](https://nl.aliexpress.com/item/1005001771511348.html)
- **I2C OLED screen (2.42 inch):** [AliExpress](https://nl.aliexpress.com/item/1005006345983913.html)

## Getting Started

### Prerequisites

- Windows ~~(or Linux)~~ PC
- Arduino Nano, Pro Micro, or Uno board

### Installation

1. Clone the repo
   <!-- ```sh
   git clone https://github.com/n-avontuur/deej.git
   ``` -->
<!-- 2. Download and install the latest [release](https://github.com/omriharel/deej/releases/latest). -->

## How It Works

### Hardware

The Encoders and buttons are connected to the analog pins on an Arduino board, which then connects to your PC via USB. The screen is connected using the I2C interface.

#### Schematic

I created my own PCB with the following schematic:

![Product Image](assets/schematic.png)

### Software

The Arduino board runs a C program that writes current slider and button values over its serial interface. The PC runs a lightweight Go client that reads this data and adjusts app volumes or triggers macros according to your configuration.

## Slider Mapping (Configuration)

Deej MacroPad uses a YAML configuration file named `config.yaml`, placed alongside the executable. This file determines which applications, commands and devices are mapped to which sliders and buttons.

#### EXAMPLE 
```yaml
slider_mapping:
  0: master
  1: chrome.exe
  2: spotify.exe
  3:
    - pathofexile_x64.exe
    - rocketleague.exe
  4: discord.exe
```

## Roadmap

- [X] Code for hardware 
  - [X] Add macrokeys
  - [X] Add encoders
  - [ ] Add screen 
- [ ] Two-way communication
- [ ] Create config
  - [ ] Add Commands 
  - [ ] Add Pages readable 
  - [ ] Add sharing between pc and arduino.
- [ ] Combine two-way communication and config
- [ ] Release executable 

## Build Your Own
If you want to build your own you could use the following Bill of Materials.
Also use the Arduino code that is defined in [Arduino\Screen-Encoders-Keys](arduino\Screen-Encoders-Keys) 

### Bill of Materials

- 1X Arduino Nano, Pro Micro, or Uno board
- 2X  Encoders
- 12X Buttons
- 1X I2C OLED screen
- Wires and connectors
- Enclosure (3D printed or custom-made)

<!-- ### Thingiverse Collection

Browse community-created 3D designs on [Thingiverse](https://thingiverse.com/omriharel/collections/deej). -->

<!-- ### Build Procedure

1. Connect components according to the schematic.
2. Flash the Arduino with the provided sketch.  
~~3. Run the Deej executable on your PC.~~

## How to Run 

### Windows


~~Just run the executable.~~
-->

## Contributing

Contributions are what make the open-source community such an amazing place to learn, inspire, and create. However, as a starting programmer, I am currently not accepting pull requests.

## License

Distributed under the MIT License. See `LICENSE.txt` for more information.


## Acknowledgments

- [Omri Harel's Deej project](https://github.com/omriharel/deej)

---

This README provides a comprehensive overview of the combined Deej and Deej MacroPad projects, covering all essential aspects from features and hardware components to installation and usage.


<!-- Example of making link to images -->
[product-screenshot]: images/screenshot.png

[WOKWI-url]: https://wokwi.com
[wokwi.js]: https://img.shields.io/badge/wokwi-black


[EasyEDA-url]: https://easyeda.com/
[EasyEDA.js]: https://img.shields.io/badge/EasyEDA-blue


[Arduino.js]:https://img.shields.io/badge/Arduino-green
[arduino-url]: https://www.arduino.cc/

[Go.js]:https://img.shields.io/badge/Go-blue
[GoLang-url]: https://golang.org/
