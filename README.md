# SpaceDB

SpaceDB is a C++23 learning-oriented SQL database. The project uses CMake and vcpkg.

The current stage contains design documents and an empty build skeleton. SQL,
storage, MVCC, server, client, and test implementations have intentionally not
been added yet.

## Repository rules

Read `.github/copilot-instructions.md` before changing the project. The
mandatory rules are under `.github/Guidelines/`, and the learning material is
under `Docs/`.

The source directory names intentionally use the GacUI capitalization:

```text
Source/     Production C++ source
Test/       Unit, component, and integration tests
Tools/      Repository tools
Docs/       Design and learning documents
```

## Prerequisites

- CMake 3.25 or newer
- A C++23 compiler
- Ninja or another generator supported by the selected CMake preset
- vcpkg with `VCPKG_ROOT` set

## Configure and build

From the repository root:

```powershell
cmake --preset windows-x64-debug
cmake --build --preset windows-x64-debug
ctest --preset windows-x64-debug
```

The release preset is `windows-x64-release`. The presets use the vcpkg toolchain
at `$env:VCPKG_ROOT/scripts/buildsystems/vcpkg.cmake`.

## Design and implementation order

1. Common errors, values, and key encoding
2. SQL lexer, AST, and parser
3. Schema and SQL types
4. Storage Engine, Memory Engine, and Disk Engine
5. MVCC
6. SQL Engine and catalog
7. Planner and executor
8. Session, server, and client

See [Docs/Architecture.md](Docs/Architecture.md) for dependency direction and
[Docs/TestingStrategy.md](Docs/TestingStrategy.md) for the acceptance matrix.
