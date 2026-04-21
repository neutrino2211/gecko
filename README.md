<!-- Improved compatibility of back to top link: See: https://github.com/othneildrew/Best-README-Template/pull/73 -->
<a name="readme-top"></a>
<!--
*** Thanks for checking out the Best-README-Template. If you have a suggestion
*** that would make this better, please fork the repo and create a pull request
*** or simply open an issue with the tag "enhancement".
*** Don't forget to give the project a star!
*** Thanks again! Now go create something AMAZING! :D
-->



<!-- PROJECT SHIELDS -->
<!--
*** I'm using markdown "reference style" links for readability.
*** Reference links are enclosed in brackets [ ] instead of parentheses ( ).
*** See the bottom of this document for the declaration of the reference variables
*** for contributors-url, forks-url, etc. This is an optional, concise syntax you may use.
*** https://www.markdownguide.org/basic-syntax/#reference-style-links
-->
[![Contributors][contributors-shield]][contributors-url]
[![Forks][forks-shield]][forks-url]
[![Stargazers][stars-shield]][stars-url]
[![Issues][issues-shield]][issues-url]
[![GPL-3.0 License][license-shield]][license-url]


<!-- PROJECT LOGO -->
<br />
<div align="center">
  <a href="https://github.com/neutrino2211/gecko">
    <img src="docs/GECKO.png" alt="Logo" height="80">
  </a>

<h3 align="center">Gecko</h3>

  <p align="center">
    A programming language designed for writing low level and highly performant applications using a beginner friendly syntax.
    <br />
    <a href="https://github.com/neutrino2211/gecko"><strong>Explore the docs »</strong></a>
    <br />
    <br />
    <a href="https://github.com/neutrino2211/gecko">View Demo</a>
    ·
    <a href="https://github.com/neutrino2211/gecko/issues">Report Bug</a>
    ·
    <a href="https://github.com/neutrino2211/gecko/issues">Request Feature</a>
  </p>
</div>



<!-- TABLE OF CONTENTS -->
<details>
  <summary>Table of Contents</summary>
  <ol>
    <li>
      <a href="#about-the-project">About The Project</a>
      <ul>
        <li><a href="#built-with">Built With</a></li>
      </ul>
    </li>
    <li>
      <a href="#getting-started">Getting Started</a>
      <ul>
        <li><a href="#prerequisites">Prerequisites</a></li>
        <li><a href="#installation">Installation</a></li>
      </ul>
    </li>
    <li><a href="#usage">Usage</a></li>
    <li><a href="#roadmap">Roadmap</a></li>
    <li><a href="#contributing">Contributing</a></li>
    <li><a href="#license">License</a></li>
    <li><a href="#contact">Contact</a></li>
    <li><a href="#acknowledgments">Acknowledgments</a></li>
  </ol>
</details>



<!-- ABOUT THE PROJECT -->
## About The Project

Gecko is a compiled systems programming language that combines TypeScript-like ergonomics with low-level control. It's designed to make systems programming accessible while maintaining the performance and control needed for OS kernels, embedded systems, and performance-critical applications.

**Key Features:**
- **Familiar syntax** - If you know TypeScript or Go, you'll feel at home
- **Traits & generics** - Powerful abstractions without runtime cost
- **C interoperability** - Seamless integration with existing C libraries
- **Memory safety tools** - `Box<T>`, `Rc<T>`, and `Drop` for automatic cleanup
- **Operator overloading** - Custom operators via trait hooks
- **Low-level control** - Inline assembly, volatile pointers, packed structs
- **Modern tooling** - LSP support, VS Code extension, cross-compilation

<p align="right">(<a href="#readme-top">back to top</a>)</p>


<!-- 
### Built With

* [![Next][Next.js]][Next-url]
* [![React][React.js]][React-url]
* [![Vue][Vue.js]][Vue-url]
* [![Angular][Angular.io]][Angular-url]
* [![Svelte][Svelte.dev]][Svelte-url]
* [![Laravel][Laravel.com]][Laravel-url]
* [![Bootstrap][Bootstrap.com]][Bootstrap-url]
* [![JQuery][JQuery.com]][JQuery-url]

<p align="right">(<a href="#readme-top">back to top</a>)</p> -->



<!-- GETTING STARTED -->
## Getting Started

### Prerequisites

Install go>=1.20
* go
  Go to [the golang download page](https://go.dev/doc/install) and follow the instructions
* LLVM
* GCC

### Installation

1. Clone the repo
   ```sh
   git clone https://github.com/neutrino2211/gecko.git
   ```
2. Install Go packages
   ```sh
   cd gecko && go get
   ```
3. Test run the program to make sure it is working
   ```sh
   go run .
   ```

<p align="right">(<a href="#readme-top">back to top</a>)</p>



<!-- USAGE EXAMPLES -->
## Usage

### Quick Start

```gecko
// hello.gecko
package main

import std.collections.string use { String }

func main(): void {
    let greeting = String::from("Hello, Gecko!")
    // Print using C interop
}
```

### Commands

```sh
# Compile to C code
gecko compile hello.gecko

# Build executable
gecko build hello.gecko -o hello

# Compile and run
gecko run hello.gecko

# Type-check without building
gecko check hello.gecko

# Generate documentation
gecko doc ./src
```

### Examples

Check out the `examples/` directory for complete projects:
- `examples/traits/` - Traits, generics, and trait constraints
- `examples/string_builder/` - Using the standard library
- `examples/hello_kernel/` - Bare-metal kernel development
- `examples/c_interop/` - C library integration

<p align="right">(<a href="#readme-top">back to top</a>)</p>

<!-- ROADMAP -->
## Roadmap

### Core Language
- [X] Variables (`let`, `const`) with type inference
- [X] Primitive types (`int`, `uint`, `bool`, `string`, `float`, etc.)
- [X] Functions with parameters and return types
- [X] Control flow (`if`/`else`, `while`, `for`, `for-in` loops)
- [X] Fixed-size arrays (`[N]T`)
- [X] Type casting (`as` operator)
- [X] Pointers (including non-null `T*!` and volatile `T volatile*`)

### Type System
- [X] Classes/structs with fields and methods
- [X] Traits and trait implementations
- [X] Generics with type parameters (`<T>`)
- [X] Trait constraints (`<T is Trait>`, multiple constraints)
- [X] Enums
- [X] Type checking and inference
- [X] Visibility modifiers (`public`, `protected`, `private`)

### Module System
- [X] Module imports (`import std.collections.vec`)
- [X] Selective imports (`use { Vec, String }`)
- [X] Directory imports
- [X] C header imports (`cimport`)

### Memory & Ownership
- [X] `Box<T>` - unique ownership
- [X] `Rc<T>` / `Weak<T>` - reference counting
- [X] `Drop` trait for automatic cleanup
- [X] `Clone` and `Copy` traits

### Operator Overloading
- [X] Hook attributes for operators (`@add_hook`, `@eq_hook`, etc.)
- [X] Iterator hooks for `for-in` loops
- [X] Index hooks for `[]` syntax
- [X] Error handling (`try` and `or` keywords)

### Tooling
- [X] C backend (recommended)
- [X] LLVM backend (experimental)
- [X] Cross-compilation support
- [X] LSP with completions, hover, diagnostics
- [X] VS Code extension
- [X] Project configuration (`gecko.toml`)

### Planned
- [ ] Trait inheritance
- [ ] Pattern matching for enums
- [ ] Async/await
- [ ] Package manager

See the [open issues](https://github.com/neutrino2211/gecko/issues) for a full list of proposed features (and known issues).

<p align="right">(<a href="#readme-top">back to top</a>)</p>



<!-- CONTRIBUTING -->
## Contributing

Contributions are what make the open source community such an amazing place to learn, inspire, and create. Any contributions you make are **greatly appreciated**.

If you have a suggestion that would make this better, please fork the repo and create a pull request. You can also simply open an issue with the tag "enhancement".
Don't forget to give the project a star! Thanks again!

1. Fork the Project
2. Create your Feature Branch (`git checkout -b feature/AmazingFeature`)
3. Commit your Changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the Branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

<p align="right">(<a href="#readme-top">back to top</a>)</p>



<!-- LICENSE -->
## License

Distributed under the GPL-3.0 License. See `LICENSE` for more information.

<p align="right">(<a href="#readme-top">back to top</a>)</p>



<!-- CONTACT -->
## Contact

Tsowa Mainasara - [@neutrino2211](https://twitter.com/neutrino2211) - tsowamainasara@gmail.com

Project Link: [https://github.com/neutrino2211/gecko](https://github.com/neutrino2211/gecko)

<p align="right">(<a href="#readme-top">back to top</a>)</p>



<!-- ACKNOWLEDGMENTS -->
## Acknowledgments

* [Best Read Me](https://github.com/othneildrew/Best-README-Template/tree/master)
* [Participle](https://github.com/alecthomas/participle)

<p align="right">(<a href="#readme-top">back to top</a>)</p>



<!-- MARKDOWN LINKS & IMAGES -->
<!-- https://www.markdownguide.org/basic-syntax/#reference-style-links -->
[contributors-shield]: https://img.shields.io/github/contributors/neutrino2211/gecko.svg?style=for-the-badge
[contributors-url]: https://github.com/neutrino2211/gecko/graphs/contributors
[forks-shield]: https://img.shields.io/github/forks/neutrino2211/gecko.svg?style=for-the-badge
[forks-url]: https://github.com/neutrino2211/gecko/network/members
[stars-shield]: https://img.shields.io/github/stars/neutrino2211/gecko.svg?style=for-the-badge
[stars-url]: https://github.com/neutrino2211/gecko/stargazers
[issues-shield]: https://img.shields.io/github/issues/neutrino2211/gecko.svg?style=for-the-badge
[issues-url]: https://github.com/neutrino2211/gecko/issues
[license-shield]: https://img.shields.io/github/license/neutrino2211/gecko.svg?style=for-the-badge
[license-url]: https://github.com/neutrino2211/gecko/blob/master/LICENSE.txt
[linkedin-shield]: https://img.shields.io/badge/-LinkedIn-black.svg?style=for-the-badge&logo=linkedin&colorB=555
[linkedin-url]: https://linkedin.com/in/linkedin_username
[product-screenshot]: images/screenshot.png
[Next.js]: https://img.shields.io/badge/next.js-000000?style=for-the-badge&logo=nextdotjs&logoColor=white
[Next-url]: https://nextjs.org/
[React.js]: https://img.shields.io/badge/React-20232A?style=for-the-badge&logo=react&logoColor=61DAFB
[React-url]: https://reactjs.org/
[Vue.js]: https://img.shields.io/badge/Vue.js-35495E?style=for-the-badge&logo=vuedotjs&logoColor=4FC08D
[Vue-url]: https://vuejs.org/
[Angular.io]: https://img.shields.io/badge/Angular-DD0031?style=for-the-badge&logo=angular&logoColor=white
[Angular-url]: https://angular.io/
[Svelte.dev]: https://img.shields.io/badge/Svelte-4A4A55?style=for-the-badge&logo=svelte&logoColor=FF3E00
[Svelte-url]: https://svelte.dev/
[Laravel.com]: https://img.shields.io/badge/Laravel-FF2D20?style=for-the-badge&logo=laravel&logoColor=white
[Laravel-url]: https://laravel.com
[Bootstrap.com]: https://img.shields.io/badge/Bootstrap-563D7C?style=for-the-badge&logo=bootstrap&logoColor=white
[Bootstrap-url]: https://getbootstrap.com
[JQuery.com]: https://img.shields.io/badge/jQuery-0769AD?style=for-the-badge&logo=jquery&logoColor=white
[JQuery-url]: https://jquery.com 