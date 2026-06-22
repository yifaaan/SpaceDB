cmake_minimum_required(VERSION 3.25)

project(SpaceDB VERSION 0.1.0 LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_EXTENSIONS OFF)
set(CMAKE_EXPORT_COMPILE_COMMANDS ON)

set(ABSL_PROPAGATE_CXX_STD ON)

find_package(absl CONFIG REQUIRED)
find_package(asio CONFIG REQUIRED)
find_package(cereal CONFIG REQUIRED)

option(SPACEDB_BUILD_TESTS "Build SpaceDB tests when test sources are present" ON)

add_library(SpaceDB STATIC
    Source/Sql/Parser/Token.cpp
    Source/Sql/Parser/Lexer.cpp
    Source/Sql/Parser/Parser.cpp
    Source/Sql/Plan/Planner.cpp
    Source/Sql/Plan/Plan.cpp
    Source/Sql/Executor/Executor.cpp
    Source/Sql/Engine/Engine.cpp
)
add_library(SpaceDB::SpaceDB ALIAS SpaceDB)

target_compile_features(SpaceDB PUBLIC cxx_std_20)

target_link_libraries(SpaceDB
    PUBLIC
        absl::status
        absl::statusor
        absl::strings
        asio::asio
        cereal::cereal
)

target_include_directories(SpaceDB
    PUBLIC
        $<BUILD_INTERFACE:${CMAKE_CURRENT_SOURCE_DIR}/Source>
        $<INSTALL_INTERFACE:include>
)

if(MSVC)
    target_compile_options(SpaceDB PUBLIC /utf-8)
endif()

include(CTest)

if(BUILD_TESTING AND SPACEDB_BUILD_TESTS)
    add_subdirectory(Test)
endif()
