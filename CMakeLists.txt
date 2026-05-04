cmake_minimum_required(VERSION 3.25)

project(SpaceDB VERSION 0.1.0 LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 23)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_EXTENSIONS OFF)

option(SPACEDB_BUILD_TESTS "Build SpaceDB tests when test sources are present" ON)

# The implementation is intentionally empty at this stage. Keeping the public
# target in the graph lets the first module add sources without changing the
# project-facing target name.
add_library(SpaceDB INTERFACE)
add_library(SpaceDB::SpaceDB ALIAS SpaceDB)

target_compile_features(SpaceDB INTERFACE cxx_std_23)

target_include_directories(SpaceDB
    INTERFACE
        $<BUILD_INTERFACE:${CMAKE_CURRENT_SOURCE_DIR}/Source>
        $<INSTALL_INTERFACE:include>
)

include(CTest)

if(BUILD_TESTING AND SPACEDB_BUILD_TESTS)
    message(STATUS "SpaceDB tests are not implemented yet")
endif()
