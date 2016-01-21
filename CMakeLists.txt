CMAKE_MINIMUM_REQUIRED(VERSION 2.8.2)
PROJECT(backrunner)

SET(BMETA_NAME backrunner_meta)

SET(MAIN_NAME backrunner)

MACRO(LOCATE_PATH VARIABLE FILE_NAME)
    IF(${VARIABLE}_TMP AND ${VARIABLE}_PATH)
        RETURN()
    ENDIF()

    FIND_PATH(${VARIABLE}_TMP NAMES ${FILE_NAME})

    IF(${VARIABLE}_TMP)
        SET(${VARIABLE}_PATH ${${VARIABLE}_TMP}/${FILE_NAME})
        MESSAGE(STATUS "${FILE_NAME} at: ${${VARIABLE}_PATH}")
    ELSE()
        MESSAGE(FATAL_ERROR "${FILE_NAME} not found.")
    ENDIF()
ENDMACRO()

LOCATE_PATH(GO_MODULE_CMAKE share/sputnik/gopack/cmake)
SET(CMAKE_MODULE_PATH ${CMAKE_MODULE_PATH} ${GO_MODULE_CMAKE_PATH})

INCLUDE(FindGo2)

GoProjectConfigure(PKGNAME github.com/minaevmike/backrunner)

GoDepends()

GoBuild(TARGET bmeta
        PACKAGE github.com/bioothod/backrunner/meta
	OUTPUT ${BMETA_NAME})

GoBuild(TARGET backrunner
        PACKAGE github.com/bioothod/backrunner
	OUTPUT ${MAIN_NAME})
INSTALL(
	PROGRAMS ${CMAKE_CURRENT_BINARY_DIR}/${BMETA_NAME} ${CMAKE_CURRENT_BINARY_DIR}/${MAIN_NAME}
	DESTINATION bin COMPONENT runtime
)
