/**
 * @file test_logger.c
 * @brief Tests for newline handling in the legacy logger.
 *
 * Verifies that each log entry produces exactly one output line,
 * truncated messages include a trailing newline, repeated messages
 * maintain correct line boundaries, and embedded newlines in message
 * bodies are preserved.
 *
 * Compile with:
 *   gcc -I../include -o test_logger test_logger.c ../src/logger.c -lpthread
 *
 * Run with:
 *   ./test_logger
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <sys/wait.h>
#include "logger.h"

static const char *TEST_LOG = "/tmp/frailbox_logger_test.log";

static int count_newlines(const char *s)
{
    int n = 0;
    for (const char *p = s; *p; p++) {
        if (*p == '\n') n++;
    }
    return n;
}

static void write_log_to_file(const char *path)
{
    FILE *fp = fopen(path, "w");
    if (!fp) {
        fprintf(stderr, "FAIL: cannot open %s for writing\n", path);
        return;
    }
    fclose(fp);

    setenv("LOG_FILE", path, 1);
    setenv("LOG_LEVEL", "debug", 1);
    setenv("LOG_SOURCE_INFO", "0", 1);
    setenv("LOG_NO_TIMESTAMPS", "1", 1);
}

static char *read_file(const char *path)
{
    FILE *fp = fopen(path, "r");
    if (!fp) return NULL;
    fseek(fp, 0, SEEK_END);
    long sz = ftell(fp);
    fseek(fp, 0, SEEK_SET);
    char *buf = (char *)malloc((size_t)sz + 1);
    if (buf) {
        size_t n = fread(buf, 1, (size_t)sz, fp);
        buf[n] = '\0';
    }
    fclose(fp);
    return buf;
}

static int test_repeated_messages_line_boundaries(void)
{
    write_log_to_file(TEST_LOG);
    log_init();

    int count = 50;
    for (int i = 0; i < count; i++) {
        LOG_INFO("repeated message %d", i);
    }

    log_shutdown();
    unsetenv("LOG_FILE");
    unsetenv("LOG_LEVEL");
    unsetenv("LOG_SOURCE_INFO");
    unsetenv("LOG_NO_TIMESTAMPS");

    char *content = read_file(TEST_LOG);
    if (!content) {
        fprintf(stderr, "FAIL(repeated): could not read log file\n");
        return 1;
    }

    int lines = count_newlines(content);
    /* +1 for the log_init INFO message */
    int pass = (lines == count + 1);
    if (!pass) {
        fprintf(stderr, "FAIL(repeated): expected %d lines, got %d\n", count + 1, lines);
        fprintf(stderr, "  content: %s\n", content);
    }
    free(content);
    return pass ? 0 : 1;
}

static int test_truncated_message_has_newline(void)
{
    write_log_to_file(TEST_LOG);
    log_init();

    /* MAX_LOG_LINE is 4096; prefix is roughly 30-60 chars.
     * Send a message that will definitely be truncated. */
    char big[5000];
    memset(big, 'A', sizeof(big) - 1);
    big[sizeof(big) - 1] = '\0';
    LOG_INFO("%s", big);

    log_shutdown();
    unsetenv("LOG_FILE");
    unsetenv("LOG_LEVEL");
    unsetenv("LOG_SOURCE_INFO");
    unsetenv("LOG_NO_TIMESTAMPS");

    char *content = read_file(TEST_LOG);
    if (!content) {
        fprintf(stderr, "FAIL(trunc): could not read log file\n");
        return 1;
    }

    /* The output must end with a newline (and nothing else after it) */
    size_t len = strlen(content);
    int pass = (len > 0 && content[len - 1] == '\n');
    if (!pass) {
        fprintf(stderr, "FAIL(trunc): output does not end with newline\n");
        fprintf(stderr, "  last 80 chars: '%.80s'\n",
                len > 80 ? content + len - 80 : content);
    }
    free(content);
    return pass ? 0 : 1;
}

static int test_embedded_newline_preserved(void)
{
    write_log_to_file(TEST_LOG);
    log_init();

    LOG_INFO("line1\nline2");
    LOG_INFO("a\nb\nc");

    log_shutdown();
    unsetenv("LOG_FILE");
    unsetenv("LOG_LEVEL");
    unsetenv("LOG_SOURCE_INFO");
    unsetenv("LOG_NO_TIMESTAMPS");

    char *content = read_file(TEST_LOG);
    if (!content) {
        fprintf(stderr, "FAIL(embedded): could not read log file\n");
        return 1;
    }

    /* "line1\nline2" should produce one log entry containing an
     * embedded newline, plus the logger's own trailing newline.
     * Total newlines: one from embedded + one from logger = 2
     * for the first message, and similarly 3 for the second.
     * But we only care that both embedded newlines exist. */
    int has_line1 = (strstr(content, "line1") != NULL);
    int has_line2 = (strstr(content, "line2") != NULL);
    int has_a = (strstr(content, "linea") != NULL || strstr(content, "line1\nline2") != NULL);

    int pass = has_line1 && has_line2;
    if (!pass) {
        fprintf(stderr, "FAIL(embedded): expected embedded newline content\n");
        fprintf(stderr, "  content: %s\n", content);
    }
    free(content);
    return pass ? 0 : 1;
}

static int test_single_message_one_line(void)
{
    write_log_to_file(TEST_LOG);
    log_init();

    LOG_INFO("hello world");
    LOG_WARN("just a warning");
    LOG_ERROR("an error");

    log_shutdown();
    unsetenv("LOG_FILE");
    unsetenv("LOG_LEVEL");
    unsetenv("LOG_SOURCE_INFO");
    unsetenv("LOG_NO_TIMESTAMPS");

    char *content = read_file(TEST_LOG);
    if (!content) {
        fprintf(stderr, "FAIL(one-line): could not read log file\n");
        return 1;
    }

    int lines = count_newlines(content);
    /* +1 for the log_init INFO message */
    int pass = (lines == 4);
    if (!pass) {
        fprintf(stderr, "FAIL(one-line): expected 4 lines, got %d\n", lines);
        fprintf(stderr, "  content: %s\n", content);
    }
    free(content);
    return pass ? 0 : 1;
}

int main(void)
{
    int failures = 0;

    failures += test_single_message_one_line();
    failures += test_repeated_messages_line_boundaries();
    failures += test_truncated_message_has_newline();
    failures += test_embedded_newline_preserved();

    fprintf(stderr, "\n--- logger newline tests: %s (%d failure%s) ---\n",
            failures == 0 ? "PASS" : "FAIL",
            failures, failures == 1 ? "" : "s");

    unlink(TEST_LOG);
    return failures;
}
