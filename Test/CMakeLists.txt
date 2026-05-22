find_package(Catch2 3 CONFIG REQUIRED)

include(Catch)

add_executable(SpaceDBLexerTest
    Sql/Parser/LexerTest.cpp
)

target_link_libraries(SpaceDBLexerTest
    PRIVATE
        SpaceDB::SpaceDB
        Catch2::Catch2WithMain
)

catch_discover_tests(SpaceDBLexerTest)

add_executable(SpaceDBAstTest
    Sql/Parser/AstTest.cpp
)

target_link_libraries(SpaceDBAstTest
    PRIVATE
        SpaceDB::SpaceDB
        Catch2::Catch2WithMain
)

catch_discover_tests(SpaceDBAstTest)

add_executable(SpaceDBParserTest
    Sql/Parser/ParserTest.cpp
)

target_link_libraries(SpaceDBParserTest
    PRIVATE
        SpaceDB::SpaceDB
        Catch2::Catch2WithMain
)

catch_discover_tests(SpaceDBParserTest)