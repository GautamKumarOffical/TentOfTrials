#include "../include/buddy.h"

#include <stdio.h>

#define CHECK(cond, msg) do { \
    if (!(cond)) { \
        fprintf(stderr, "check failed: %s\n", msg); \
        return 1; \
    } \
} while (0)

static int test_alloc_free_and_stats(void) {
    CHECK(buddy_init(4096) == 0, "buddy_init should create a 4096 byte heap");

    void *first = buddy_alloc(17);
    void *second = buddy_alloc(80);
    CHECK(first != NULL, "first allocation should succeed");
    CHECK(second != NULL, "second allocation should succeed");
    CHECK(first != second, "allocations should return distinct blocks");

    buddy_stats_t stats = buddy_stats();
    CHECK(stats.heap_size == 4096, "heap size should be tracked");
    CHECK(stats.allocation_count == 2, "allocation count should be tracked");
    CHECK(stats.current_usage >= 97, "current requested usage should be tracked");
    CHECK(stats.peak_usage >= stats.current_usage, "peak usage should cover current usage");
    CHECK(stats.allocated_blocks == 2, "allocated block count should be tracked");

    buddy_free(first);
    buddy_free(second);

    stats = buddy_stats();
    CHECK(stats.deallocation_count == 2, "free count should be tracked");
    CHECK(stats.current_usage == 0, "usage should return to zero after frees");
    CHECK(stats.free_blocks == 1, "freed buddies should coalesce to one block");
    CHECK(stats.largest_free_block == stats.heap_size, "coalesced heap should be the largest free block");
    CHECK(stats.fragmentation_ratio == 0.0, "single free block should have zero fragmentation");

    buddy_shutdown();
    return 0;
}

static int test_fragmentation_and_oversized_alloc(void) {
    CHECK(buddy_init(4096) == 0, "buddy_init should reset allocator state");

    void *left = buddy_alloc(128);
    void *middle = buddy_alloc(128);
    void *right = buddy_alloc(128);
    CHECK(left != NULL, "left allocation should succeed");
    CHECK(middle != NULL, "middle allocation should succeed");
    CHECK(right != NULL, "right allocation should succeed");

    buddy_free(middle);
    buddy_stats_t stats = buddy_stats();
    CHECK(stats.fragmentation_ratio >= 0.0, "fragmentation ratio should not be negative");
    CHECK(stats.fragmentation_ratio <= 1.0, "fragmentation ratio should not exceed one");
    CHECK(buddy_alloc(8192) == NULL, "oversized allocation should fail");

    buddy_free(left);
    buddy_free(right);
    buddy_shutdown();
    return 0;
}

int main(void) {
    if (test_alloc_free_and_stats() != 0) {
        return 1;
    }
    if (test_fragmentation_and_oversized_alloc() != 0) {
        return 1;
    }

    puts("buddy allocator tests passed");
    return 0;
}
