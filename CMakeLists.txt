cmake_minimum_required(VERSION 3.25)

# The application is a Go program using MIQT's Qt 6 bindings. CMake orchestrates
# Go compilation and Qt runtime deployment; it does not compile Go as C++.
project(InvoiceMakerDev LANGUAGES NONE)

set(QT_ROOT "G:/qt/6.12.0/mingw_64" CACHE PATH "Qt 6.12.0 MinGW kit root")

find_program(GO_EXECUTABLE NAMES go REQUIRED)
if(WIN32)
    find_program(WINDEPLOYQT_EXECUTABLE
        NAMES windeployqt.exe
        PATHS "${QT_ROOT}/bin"
        NO_DEFAULT_PATH
    )
    if(NOT WINDEPLOYQT_EXECUTABLE)
        message(FATAL_ERROR
            "windeployqt.exe was not found under '${QT_ROOT}/bin'. "
            "Set QT_ROOT to the Qt 6.12.0 MinGW kit."
        )
    endif()
    set(APP_NAME "invoice-maker.exe")
else()
    set(APP_NAME "invoice-maker")
endif()

set(APP_OUTPUT_DIR "${CMAKE_BINARY_DIR}/bin")
set(APP_EXECUTABLE "${APP_OUTPUT_DIR}/${APP_NAME}")
set(HESORA_LICENSE_PUBLIC_KEY "$ENV{HESORA_LICENSE_PUBLIC_KEY}" CACHE STRING "Base64url Ed25519 public key for license verification")
set(HESORA_GO_LDFLAGS "")
if(WIN32)
    set(HESORA_GO_LDFLAGS "-H=windowsgui")
endif()
if(HESORA_LICENSE_PUBLIC_KEY)
    if(NOT HESORA_LICENSE_PUBLIC_KEY MATCHES "^[A-Za-z0-9_+/=-]+$")
        message(FATAL_ERROR "HESORA_LICENSE_PUBLIC_KEY must be a base64 Ed25519 public key")
    endif()
    string(APPEND HESORA_GO_LDFLAGS " -X main.hesoraLicensePublicKeyBase64=${HESORA_LICENSE_PUBLIC_KEY}")
endif()

file(GLOB_RECURSE GO_SOURCES CONFIGURE_DEPENDS
    "${CMAKE_SOURCE_DIR}/*.go"
)
file(GLOB_RECURSE FONT_ASSETS CONFIGURE_DEPENDS
    "${CMAKE_SOURCE_DIR}/assets/font/*"
)
set(APP_DEPENDENCIES
    ${GO_SOURCES}
    ${FONT_ASSETS}
    "${CMAKE_SOURCE_DIR}/go.mod"
    "${CMAKE_SOURCE_DIR}/go.sum"
    "${CMAKE_SOURCE_DIR}/assets/invoice-maker.ico"
)

if(WIN32)
    add_custom_command(
        OUTPUT "${APP_EXECUTABLE}"
        COMMAND "${CMAKE_COMMAND}" -E make_directory "${APP_OUTPUT_DIR}"
        COMMAND "${GO_EXECUTABLE}" build -trimpath "-ldflags=${HESORA_GO_LDFLAGS}"
                -o "${APP_EXECUTABLE}" ./cmd/invoice-maker
        COMMAND "${WINDEPLOYQT_EXECUTABLE}" --release --compiler-runtime
                --no-translations --dir "${APP_OUTPUT_DIR}" "${APP_EXECUTABLE}"
        COMMAND "${CMAKE_COMMAND}" -E make_directory "${APP_OUTPUT_DIR}/assets"
        COMMAND "${CMAKE_COMMAND}" -E copy_if_different
                "${CMAKE_SOURCE_DIR}/assets/invoice-maker.ico"
                "${APP_OUTPUT_DIR}/assets/invoice-maker.ico"
        COMMAND "${CMAKE_COMMAND}" -E copy_directory
                "${CMAKE_SOURCE_DIR}/assets/font"
                "${APP_OUTPUT_DIR}/assets/font"
        DEPENDS ${APP_DEPENDENCIES}
        WORKING_DIRECTORY "${CMAKE_SOURCE_DIR}"
        COMMENT "Building Invoice Maker and deploying the Qt runtime"
        VERBATIM
    )
else()
    add_custom_command(
        OUTPUT "${APP_EXECUTABLE}"
        COMMAND "${CMAKE_COMMAND}" -E make_directory "${APP_OUTPUT_DIR}"
        COMMAND "${GO_EXECUTABLE}" build -trimpath "-ldflags=${HESORA_GO_LDFLAGS}"
                -o "${APP_EXECUTABLE}" ./cmd/invoice-maker
        DEPENDS ${APP_DEPENDENCIES}
        WORKING_DIRECTORY "${CMAKE_SOURCE_DIR}"
        COMMENT "Building Invoice Maker"
        VERBATIM
    )
endif()

add_custom_target(build-app DEPENDS "${APP_EXECUTABLE}")

# Build and launch directly from the development output directory. No MSI install
# or uninstall is required for normal code/UI iteration.
add_custom_target(run-app
    COMMAND "${APP_EXECUTABLE}"
    DEPENDS build-app
    WORKING_DIRECTORY "${APP_OUTPUT_DIR}"
    USES_TERMINAL
    COMMENT "Launching Invoice Maker from the build directory"
)
