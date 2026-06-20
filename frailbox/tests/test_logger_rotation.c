/**
 * @file test_logger_rotation.c
 * @brief Test suite for the logger file rotation feature.
 *
 * This test suite verifies that the legacy logger correctly rotates
 * log files when they exceed the configured maximum size.
 *
 * Compile with:
 *   gcc -I.. -o test_logger_rotation test_logger_rotation.c ../src/logger.c -lpthread
 *
 * Run with:
 *   ./test_logger_rotation
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <setjmp.h>
#include <time.h>
#include <sys/stat.h>
#include <unistd.h>
#include <errno.h>

#include "../include/logger.h"

/* ================================================================== */
/* MINIMAL TEST FRAMEWORK                                             */
/* ================================================================== */

#define MAX_TESTS 256
#define TEST_NAME_MAX 128

typedef struct {
    const char *name;
    int (*func)(void);
    int failed;
    double duration_ms;
    const char *file;
    int line;
} test_case_t;

typedef struct {
    const char *name;
    int (*setup)(void);
    int (*teardown)(void);
} test_suite_t;

static test_case_t tests[MAX_TESTS];
static int test_count = 0;
static int tests_passed = 0;
static int tests_failed = 0;
static int tests_skipped = 0;

static jmp_buf assert_jmp;
static int assert_failed = 0;
static char assert_msg[1024];

#define TEST_SUITE(name) static int test_suite_##name = 0

#define TEST(test_id) \
    static int test_##test_id(void); \
    __attribute__((constructor)) static void register_##test_id(void) { \
        if (test_count < MAX_TESTS) { \
            tests[test_count].name = #test_id; \
            tests[test_count].func = test_##test_id; \
            tests[test_count].failed = 0; \
            tests[test_count].file = __FILE__; \
            tests[test_count].line = __LINE__; \
            test_count++; \
        } \
    } \
    static int test_##test_id(void)

#define ASSERT(cond, msg, ...) do { \
    if (!(cond)) { \
        snprintf(assert_msg, sizeof(assert_msg), "ASSERT FAILED: " msg, ##__VA_ARGS__); \
        assert_failed = 1; \
        longjmp(assert_jmp, 1); \
    } \
} while(0)

#define ASSERT_EQ(a, b, msg, ...) ASSERT((a) == (b), "Expected " msg, ##__VA_ARGS__)
#define ASSERT_NE(a, b, msg, ...) ASSERT((a) != (b), "Expected not equal: " msg, ##__VA_ARGS__)
#define ASSERT_NULL(ptr) ASSERT((ptr) == NULL, "Expected NULL pointer")
#define ASSERT_NOT_NULL(ptr) ASSERT((ptr) != NULL, "Expected non-NULL pointer")

#define RUN_TEST_SUITE(name) run_tests(#name)

static double get_time_ms(void)
{
    struct timespec ts;
    clock_gettime(CLOCK_MONOTONIC, &ts);
    return ts.tv_sec * 1000.0 + ts.tv_nsec / 1000000.0;
}

static int run_all_tests(void)
{
    printf("\n");
    printf("============================================================\n");
    printf("  LOGGER ROTATION TEST SUITE\n");
    printf("============================================================\n\n");

    for (int i = 0; i < test_count; i++) {
        test_case_t *test = &tests[i];
        printf("  [%3d/%3d] %-50s ", i + 1, test_count, test->name);

        double start = get_time_ms();

        /* Reset assertion state */
        assert_failed = 0;

        /* Run test with setjmp for assertion handling */
        if (setjmp(assert_jmp) == 0) {
            int result = test->func();
            if (result == 0) {
                tests_passed++;
                double elapsed = get_time_ms() - start;
                printf("PASS (%.1fms)\n", elapsed);
            } else {
                tests_failed++;
                test->failed = 1;
                printf("FAIL (returned %d)\n", result);
            }
        } else {
            tests_failed++;
            test->failed = 1;
            double elapsed = get_time_ms() - start;
            printf("FAIL (%.1fms)\n", elapsed);
            printf("         %s\n", assert_msg);
        }
    }

    printf("\n");
    printf("============================================================\n");
    printf("  RESULTS: %d passed, %d failed, %d skipped out of %d\n",
           tests_passed, tests_failed, tests_skipped, test_count);
    printf("============================================================\n\n");

    return tests_failed;
}

/* ================================================================== */
/* HELPER FUNCTIONS                                                    */
/* ================================================================== */

static const char *test_log_file = "/tmp/test_logger_rotation.log";

static void cleanup_test_files(void)
{
    char path[1200];
    remove(test_log_file);
    for (int i = 1; i <= 10; i++) {
        snprintf(path, sizeof(path), "%s.%d", test_log_file, i);
        remove(path);
    }
}

static long get_file_size(const char *path)
{
    struct stat st;
    if (stat(path, &st) == 0) {
        return st.st_size;
    }
    return -1;
}

/* ================================================================== */
/* TESTS                                                               */
/* ================================================================== */

TEST(test_rotation_disabled_by_default)
{
    /* Test that rotation is disabled by default */
    cleanup_test_files();
    
    setenv("LOG_FILE", test_log_file, 1);
    unsetenv("LOG_MAX_FILE_SIZE");
    unsetenv("LOG_MAX_FILES");
    
    log_init();
    
    log_rotation_config_t config;
    int result = log_get_rotation(&config);
    
    /* Rotation should be disabled by default */
    ASSERT_EQ(config.max_file_size, 0U, "max_file_size should be 0 by default");
    
    log_shutdown();
    cleanup_test_files();
    return 0;
}

TEST(test_rotation_enabled_via_env)
{
    /* Test that rotation can be enabled via environment variables */
    cleanup_test_files();
    
    setenv("LOG_FILE", test_log_file, 1);
    setenv("LOG_MAX_FILE_SIZE", "1000", 1);
    setenv("LOG_MAX_FILES", "3", 1);
    
    log_init();
    
    log_rotation_config_t config;
    int result = log_get_rotation(&config);
    
    ASSERT_EQ(result, 0, "log_get_rotation should succeed");
    ASSERT_EQ(config.max_file_size, 1000U, "max_file_size should be 1000");
    ASSERT_EQ(config.max_files, 3, "max_files should be 3");
    
    log_shutdown();
    cleanup_test_files();
    return 0;
}

TEST(test_rotation_set_rotation_api)
{
    /* Test the log_set_rotation API */
    cleanup_test_files();
    
    setenv("LOG_FILE", test_log_file, 1);
    unsetenv("LOG_MAX_FILE_SIZE");
    unsetenv("LOG_MAX_FILES");
    
    log_init();
    
    log_rotation_config_t config;
    memset(&config, 0, sizeof(config));
    config.max_file_size = 2000;
    config.max_files = 5;
    config.rotation_path = test_log_file;
    
    int result = log_set_rotation(&config);
    ASSERT_EQ(result, 0, "log_set_rotation should succeed");
    
    log_rotation_config_t retrieved;
    result = log_get_rotation(&retrieved);
    ASSERT_EQ(result, 0, "log_get_rotation should succeed");
    ASSERT_EQ(retrieved.max_file_size, 2000U, "max_file_size should be 2000");
    ASSERT_EQ(retrieved.max_files, 5, "max_files should be 5");
    
    log_shutdown();
    cleanup_test_files();
    return 0;
}

TEST(test_rotation_disable_via_api)
{
    /* Test that rotation can be disabled via API */
    cleanup_test_files();
    
    setenv("LOG_FILE", test_log_file, 1);
    setenv("LOG_MAX_FILE_SIZE", "1000", 1);
    
    log_init();
    
    /* Verify rotation is enabled */
    log_rotation_config_t config;
    log_get_rotation(&config);
    ASSERT_EQ(config.max_file_size, 1000U, "max_file_size should be 1000");
    
    /* Disable rotation */
    int result = log_set_rotation(NULL);
    ASSERT_EQ(result, 0, "log_set_rotation(NULL) should succeed");
    
    log_get_rotation(&config);
    ASSERT_EQ(config.max_file_size, 0U, "max_file_size should be 0 after disable");
    
    log_shutdown();
    cleanup_test_files();
    return 0;
}

TEST(test_rotation_performs_when_exceeded)
{
    /* Test that rotation actually happens when file size is exceeded */
    cleanup_test_files();
    
    /* Set up with small max file size (100 bytes) */
    setenv("LOG_FILE", test_log_file, 1);
    setenv("LOG_MAX_FILE_SIZE", "100", 1);
    setenv("LOG_MAX_FILES", "3", 1);
    
    log_init();
    
    /* Write enough data to trigger rotation */
    for (int i = 0; i < 20; i++) {
        LOG_INFO("Test log message %d with some padding to fill up the file", i);
    }
    
    /* Check that rotation happened */
    long original_size = get_file_size(test_log_file);
    long rotated_size = get_file_size(test_log_file);
    char rotated_path[1200];
    snprintf(rotated_path, sizeof(rotated_path), "%s.1", test_log_file);
    long rotated_1_size = get_file_size(rotated_path);
    
    /* At least one rotated file should exist */
    ASSERT(rotated_1_size >= 0, "Rotated file .1 should exist");
    
    log_shutdown();
    cleanup_test_files();
    return 0;
}

TEST(test_rotation_preserves_existing_behavior)
{
    /* Test that existing behavior is preserved when rotation is not configured */
    cleanup_test_files();
    
    setenv("LOG_FILE", test_log_file, 1);
    unsetenv("LOG_MAX_FILE_SIZE");
    unsetenv("LOG_MAX_FILES");
    
    log_init();
    
    /* Write some log messages */
    LOG_INFO("Test message 1");
    LOG_WARN("Test message 2");
    LOG_ERROR("Test message 3");
    
    log_shutdown();
    
    /* Verify file was created and has content */
    long size = get_file_size(test_log_file);
    ASSERT(size > 0, "Log file should have content");
    
    cleanup_test_files();
    return 0;
}

TEST(test_rotation_max_files_limit)
{
    /* Test that max_files limit is respected */
    cleanup_test_files();
    
    /* Set up with small max file size and max_files=2 */
    setenv("LOG_FILE", test_log_file, 1);
    setenv("LOG_MAX_FILE_SIZE", "50", 1);
    setenv("LOG_MAX_FILES", "2", 1);
    
    log_init();
    
    /* Write enough data to trigger multiple rotations */
    for (int i = 0; i < 30; i++) {
        LOG_INFO("Test log message %d with padding to exceed rotation threshold", i);
    }
    
    /* Check that we don't have more than max_files rotated files */
    char path[1200];
    long size_1 = get_file_size(test_log_file);
    snprintf(path, sizeof(path), "%s.1", test_log_file);
    long size_rot1 = get_file_size(path);
    snprintf(path, sizeof(path), "%s.2", test_log_file);
    long size_rot2 = get_file_size(path);
    snprintf(path, sizeof(path), "%s.3", test_log_file);
    long size_rot3 = get_file_size(path);
    
    /* Should have at most 2 rotated files (.1 and .2) */
    ASSERT(size_rot3 < 0, "File .3 should not exist when max_files=2");
    
    log_shutdown();
    cleanup_test_files();
    return 0;
}

TEST(test_rotation_manual_trigger)
{
    /* Test manual rotation via log_rotate() */
    cleanup_test_files();
    
    setenv("LOG_FILE", test_log_file, 1);
    setenv("LOG_MAX_FILE_SIZE", "1000", 1);
    setenv("LOG_MAX_FILES", "3", 1);
    
    log_init();
    
    /* Write some data */
    LOG_INFO("Test message before manual rotation");
    
    /* Manually trigger rotation */
    int result = log_rotate();
    ASSERT_EQ(result, 0, "log_rotate should succeed");
    
    /* Check that rotated file was created */
    char rotated_path[1200];
    snprintf(rotated_path, sizeof(rotated_path), "%s.1", test_log_file);
    long rotated_size = get_file_size(rotated_path);
    ASSERT(rotated_size >= 0, "Rotated file .1 should exist after manual rotation");
    
    log_shutdown();
    cleanup_test_files();
    return 0;
}

TEST(test_rotation_min_files)
{
    /* Test that max_files cannot be set below 1 */
    cleanup_test_files();
    
    setenv("LOG_FILE", test_log_file, 1);
    unsetenv("LOG_MAX_FILE_SIZE");
    unsetenv("LOG_MAX_FILES");
    
    log_init();
    
    log_rotation_config_t config;
    memset(&config, 0, sizeof(config));
    config.max_file_size = 1000;
    config.max_files = 0; /* Invalid: should be clamped to 1 */
    
    int result = log_set_rotation(&config);
    ASSERT_EQ(result, 0, "log_set_rotation should succeed");
    
    log_rotation_config_t retrieved;
    log_get_rotation(&retrieved);
    ASSERT_EQ(retrieved.max_files, 1, "max_files should be clamped to 1");
    
    log_shutdown();
    cleanup_test_files();
    return 0;
}

/* ================================================================== */
/* MAIN                                                                */
/* ================================================================== */

int main(void)
{
    printf("Logger Rotation Test Suite\n\n");

    int result = run_all_tests();

    return result;
}
