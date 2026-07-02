#include "buddy.h"

#include <assert.h>
#include <math.h>
#include <stdio.h>
#include <stdint.h>
#include <string.h>

static void test_minimum_capacity_and_stats(void) {
    buddy_allocator_t *allocator = buddy_create(1u);
    assert(allocator != NULL);

    buddy_stats_t stats = buddy_stats(allocator);
    assert(stats.capacity_bytes == BUDDY_MIN_BLOCK_SIZE);
    assert(stats.free_bytes == BUDDY_MIN_BLOCK_SIZE);
    assert(stats.largest_free_block == BUDDY_MIN_BLOCK_SIZE);
    assert(stats.fragmentation_ratio == 0.0);

    buddy_destroy(allocator);
}

static void test_alloc_write_and_free(void) {
    buddy_allocator_t *allocator = buddy_create(1024u);
    assert(allocator != NULL);

    char *payload = buddy_alloc(allocator, 20u);
    assert(payload != NULL);
    assert(buddy_contains(allocator, payload));

    strcpy(payload, "frailbox buddy");
    assert(strcmp(payload, "frailbox buddy") == 0);

    buddy_stats_t stats = buddy_stats(allocator);
    assert(stats.allocated_bytes == 20u);
    assert(stats.reserved_bytes >= BUDDY_MIN_BLOCK_SIZE);
    assert(stats.allocation_count == 1u);
    assert(stats.free_bytes < stats.capacity_bytes);

    buddy_free(allocator, payload);

    stats = buddy_stats(allocator);
    assert(stats.allocated_bytes == 0u);
    assert(stats.reserved_bytes == 0u);
    assert(stats.deallocation_count == 1u);
    assert(stats.free_bytes == stats.capacity_bytes);
    assert(stats.largest_free_block == stats.capacity_bytes);
    assert(stats.coalesce_count > 0u);

    buddy_destroy(allocator);
}

static void test_fragmentation_and_reuse(void) {
    buddy_allocator_t *allocator = buddy_create(2048u);
    assert(allocator != NULL);

    void *a = buddy_alloc(allocator, 128u);
    void *b = buddy_alloc(allocator, 128u);
    void *c = buddy_alloc(allocator, 128u);
    assert(a && b && c);

    buddy_free(allocator, b);
    buddy_stats_t fragmented = buddy_stats(allocator);
    assert(fragmented.fragmentation_ratio >= 0.0);
    assert(fragmented.fragmentation_ratio <= 1.0);

    void *d = buddy_alloc(allocator, 96u);
    assert(d == b);

    buddy_free(allocator, a);
    buddy_free(allocator, c);
    buddy_free(allocator, d);

    buddy_stats_t final = buddy_stats(allocator);
    assert(final.free_bytes == final.capacity_bytes);
    assert(final.largest_free_block == final.capacity_bytes);

    buddy_destroy(allocator);
}

static void test_oversized_allocation_fails(void) {
    buddy_allocator_t *allocator = buddy_create(512u);
    assert(allocator != NULL);

    void *payload = buddy_alloc(allocator, 1024u);
    assert(payload == NULL);

    payload = buddy_alloc(allocator, SIZE_MAX);
    assert(payload == NULL);

    buddy_stats_t stats = buddy_stats(allocator);
    assert(stats.allocation_count == 0u);
    assert(stats.allocated_bytes == 0u);

    buddy_destroy(allocator);
}

int main(void) {
    test_minimum_capacity_and_stats();
    test_alloc_write_and_free();
    test_fragmentation_and_reuse();
    test_oversized_allocation_fails();

    puts("buddy allocator tests passed");
    return 0;
}
