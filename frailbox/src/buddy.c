#include "buddy.h"
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <math.h>
#include <sys/mman.h>

#define MIN_BLOCK_SIZE 64
#define MAX_ORDER 20

typedef struct block {
    int order;
    int free;
    struct block *next;
} block_t;

struct buddy_allocator {
    void *base;
    size_t total_size;
    size_t total_blocks;
    block_t *free_lists[MAX_ORDER + 1];
    buddy_stats_t stats;
};

static size_t next_power_of_two(size_t v) {
    v--;
    v |= v >> 1;
    v |= v >> 2;
    v |= v >> 4;
    v |= v >> 8;
    v |= v >> 16;
    v |= v >> 32;
    v++;
    return v;
}

static int get_order(size_t size) {
    int order = 0;
    size_t block_size = MIN_BLOCK_SIZE;
    while (block_size < size && order < MAX_ORDER) {
        block_size *= 2;
        order++;
    }
    return order;
}

static size_t block_size(int order) {
    return MIN_BLOCK_SIZE << order;
}

static block_t *get_block(void *ptr) {
    return (block_t *)((char *)ptr - sizeof(block_t));
}

static void *get_ptr(block_t *block) {
    return (void *)((char *)block + sizeof(block_t));
}

static void split_block(buddy_allocator_t *buddy, int order) {
    if (order == 0 || !buddy->free_lists[order]) return;

    block_t *block = buddy->free_lists[order];
    buddy->free_lists[order] = block->next;

    int split_order = order - 1;
    size_t half_size = block_size(split_order);

    block_t *buddy_block = (block_t *)((char *)block + half_size);
    buddy_block->order = split_order;
    buddy_block->free = 1;

    block->order = split_order;
    block->free = 1;

    block->next = buddy_block;
    buddy_block->next = buddy->free_lists[split_order];
    buddy->free_lists[split_order] = block;

    buddy->stats.block_count++;
    buddy->stats.free_block_count++;
}

static void coalesce_block(buddy_allocator_t *buddy, block_t *block) {
    int order = block->order;
    while (order < MAX_ORDER) {
        size_t bsize = block_size(order);
        uintptr_t offset = (uintptr_t)block - (uintptr_t)buddy->base;
        uintptr_t buddy_offset = offset ^ bsize;

        if (buddy_offset + bsize > buddy->total_size) break;

        block_t *buddy_block = (block_t *)((char *)buddy->base + buddy_offset);
        if (!buddy_block->free || buddy_block->order != order) break;

        block_t **prev = &buddy->free_lists[order];
        while (*prev && *prev != buddy_block) {
            prev = &(*prev)->next;
        }
        if (*prev == buddy_block) {
            *prev = buddy_block->next;
            block->order = order + 1;
            order++;
            buddy->stats.block_count--;
            buddy->stats.free_block_count--;
        } else {
            break;
        }
    }
}

buddy_allocator_t *buddy_create(size_t total_size) {
    total_size = next_power_of_two(total_size);
    if (total_size < MIN_BLOCK_SIZE * 2) {
        total_size = MIN_BLOCK_SIZE * 2;
    }

    void *base = mmap(NULL, total_size, PROT_READ | PROT_WRITE,
                       MAP_PRIVATE | MAP_ANONYMOUS, -1, 0);
    if (base == MAP_FAILED) return NULL;

    buddy_allocator_t *buddy = calloc(1, sizeof(buddy_allocator_t));
    if (!buddy) {
        munmap(base, total_size);
        return NULL;
    }

    buddy->base = base;
    buddy->total_size = total_size;
    buddy->total_blocks = total_size / MIN_BLOCK_SIZE;

    int order = get_order(total_size);
    block_t *block = (block_t *)base;
    block->order = order;
    block->free = 1;
    block->next = NULL;

    buddy->free_lists[order] = block;
    buddy->stats.block_count = 1;
    buddy->stats.free_block_count = 1;

    return buddy;
}

void buddy_destroy(buddy_allocator_t *buddy) {
    if (!buddy) return;
    if (buddy->base) {
        munmap(buddy->base, buddy->total_size);
    }
    free(buddy);
}

void *buddy_alloc(buddy_allocator_t *buddy, size_t size) {
    if (!buddy || size == 0) return NULL;

    size += sizeof(block_t);
    int order = get_order(size);

    int current_order = order;
    while (current_order <= MAX_ORDER && !buddy->free_lists[current_order]) {
        current_order++;
    }

    if (current_order > MAX_ORDER) return NULL;

    for (int i = current_order; i > order; i--) {
        split_block(buddy, i);
    }

    block_t *block = buddy->free_lists[order];
    buddy->free_lists[order] = block->next;

    block->free = 0;
    block->order = order;

    buddy->stats.block_count--;
    buddy->stats.free_block_count--;
    buddy->stats.allocation_count++;
    buddy->stats.total_allocated += block_size(order);
    buddy->stats.current_usage += block_size(order);

    if (buddy->stats.current_usage > buddy->stats.peak_usage) {
        buddy->stats.peak_usage = buddy->stats.current_usage;
    }

    return get_ptr(block);
}

void buddy_free(buddy_allocator_t *buddy, void *ptr) {
    if (!buddy || !ptr) return;

    block_t *block = get_block(ptr);
    if (block->free) return;

    block->free = 1;
    buddy->stats.deallocation_count++;
    buddy->stats.total_freed += block_size(block->order);
    buddy->stats.current_usage -= block_size(block->order);
    buddy->stats.block_count++;
    buddy->stats.free_block_count++;

    block->next = buddy->free_lists[block->order];
    buddy->free_lists[block->order] = block;

    coalesce_block(buddy, block);
}

buddy_stats_t buddy_stats(const buddy_allocator_t *buddy) {
    if (!buddy) {
        buddy_stats_t empty = {0};
        return empty;
    }
    buddy_stats_t stats = buddy->stats;
    if (stats.total_allocated > 0) {
        stats.fragmentation_ratio = (double)stats.free_block_count /
                                   (double)stats.block_count;
    }
    return stats;
}

size_t buddy_total_capacity(const buddy_allocator_t *buddy) {
    return buddy ? buddy->total_size : 0;
}

int buddy_contains(const buddy_allocator_t *buddy, const void *ptr) {
    if (!buddy || !ptr) return 0;
    return ptr >= buddy->base &&
           ptr < (char *)buddy->base + buddy->total_size;
}
