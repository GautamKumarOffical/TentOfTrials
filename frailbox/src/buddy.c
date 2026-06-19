#include "buddy.h"

#include <stdint.h>
#include <string.h>
#include <sys/mman.h>

#define BUDDY_MAX_ORDER 56U

typedef struct buddy_block {
    struct buddy_block *next;
    size_t              requested_size;
    unsigned int        order;
    int                 is_free;
} buddy_block_t;

typedef struct buddy_allocator {
    void          *heap;
    size_t         heap_size;
    unsigned int   max_order;
    buddy_block_t *free_lists[BUDDY_MAX_ORDER + 1U];
    buddy_stats_t  stats;
    int            initialized;
} buddy_allocator_t;

static buddy_allocator_t allocator;

static int is_power_of_two(size_t value) {
    return value != 0 && (value & (value - 1U)) == 0;
}

static size_t next_power_of_two(size_t value) {
    size_t power = BUDDY_MIN_BLOCK_SIZE;

    while (power < value) {
        if (power > SIZE_MAX / 2U) {
            return 0;
        }
        power <<= 1U;
    }

    return power;
}

static size_t block_size_for_order(unsigned int order) {
    return (size_t)BUDDY_MIN_BLOCK_SIZE << order;
}

static unsigned int order_for_block_size(size_t size) {
    unsigned int order = 0;

    while (size > BUDDY_MIN_BLOCK_SIZE) {
        size >>= 1U;
        order++;
    }

    return order;
}

static unsigned int order_for_request(size_t size) {
    size_t needed = size + sizeof(buddy_block_t);
    size_t block_size = next_power_of_two(needed);

    if (block_size == 0) {
        return BUDDY_MAX_ORDER + 1U;
    }

    return order_for_block_size(block_size);
}

static void insert_free_block(buddy_block_t *block) {
    unsigned int order = block->order;

    block->is_free = 1;
    block->requested_size = 0;
    block->next = allocator.free_lists[order];
    allocator.free_lists[order] = block;
}

static int remove_free_block(buddy_block_t *block, unsigned int order) {
    buddy_block_t **cursor = &allocator.free_lists[order];

    while (*cursor != NULL) {
        if (*cursor == block) {
            *cursor = block->next;
            block->next = NULL;
            return 1;
        }
        cursor = &(*cursor)->next;
    }

    return 0;
}

static int block_in_heap(const buddy_block_t *block) {
    const char *start = allocator.heap;
    const char *end = start + allocator.heap_size;
    const char *addr = (const char *)block;

    return addr >= start && addr < end;
}

static buddy_block_t *buddy_for_block(buddy_block_t *block) {
    size_t block_size = block_size_for_order(block->order);
    uintptr_t heap_addr = (uintptr_t)allocator.heap;
    uintptr_t block_addr = (uintptr_t)block;
    uintptr_t offset = block_addr - heap_addr;
    uintptr_t buddy_offset = offset ^ block_size;

    if (buddy_offset >= allocator.heap_size) {
        return NULL;
    }

    return (buddy_block_t *)(heap_addr + buddy_offset);
}

int buddy_init(size_t heap_size) {
    buddy_block_t *initial;

    if (allocator.initialized) {
        buddy_shutdown();
    }

    if (heap_size == 0) {
        heap_size = BUDDY_DEFAULT_HEAP_SIZE;
    }
    if (!is_power_of_two(heap_size)) {
        heap_size = next_power_of_two(heap_size);
    }
    if (heap_size < BUDDY_MIN_BLOCK_SIZE) {
        heap_size = BUDDY_MIN_BLOCK_SIZE;
    }

    if (heap_size == 0 || order_for_block_size(heap_size) > BUDDY_MAX_ORDER) {
        return -1;
    }

    allocator.heap = mmap(NULL, heap_size, PROT_READ | PROT_WRITE,
                          MAP_PRIVATE | MAP_ANONYMOUS, -1, 0);
    if (allocator.heap == MAP_FAILED) {
        allocator.heap = NULL;
        return -1;
    }

    allocator.heap_size = heap_size;
    allocator.max_order = order_for_block_size(heap_size);
    allocator.initialized = 1;
    memset(&allocator.stats, 0, sizeof(allocator.stats));
    memset(allocator.free_lists, 0, sizeof(allocator.free_lists));

    initial = allocator.heap;
    initial->next = NULL;
    initial->requested_size = 0;
    initial->order = allocator.max_order;
    initial->is_free = 1;
    allocator.free_lists[allocator.max_order] = initial;
    allocator.stats.heap_size = heap_size;
    allocator.stats.largest_free_block = heap_size;
    allocator.stats.free_blocks = 1;

    return 0;
}

void buddy_shutdown(void) {
    if (!allocator.initialized) {
        return;
    }

    (void)munmap(allocator.heap, allocator.heap_size);
    memset(&allocator, 0, sizeof(allocator));
}

void *buddy_alloc(size_t size) {
    buddy_block_t *block;
    unsigned int order;
    unsigned int current_order;

    if (size == 0 || size > SIZE_MAX - sizeof(buddy_block_t)) {
        return NULL;
    }

    if (!allocator.initialized && buddy_init(0) != 0) {
        return NULL;
    }

    order = order_for_request(size);
    if (order > allocator.max_order) {
        return NULL;
    }

    current_order = order;
    while (current_order <= allocator.max_order &&
           allocator.free_lists[current_order] == NULL) {
        current_order++;
    }
    if (current_order > allocator.max_order) {
        return NULL;
    }

    block = allocator.free_lists[current_order];
    (void)remove_free_block(block, current_order);

    while (current_order > order) {
        buddy_block_t *split_buddy;
        size_t split_size;

        current_order--;
        split_size = block_size_for_order(current_order);
        split_buddy = (buddy_block_t *)((char *)block + split_size);
        split_buddy->order = current_order;
        split_buddy->is_free = 1;
        split_buddy->requested_size = 0;
        split_buddy->next = NULL;

        block->order = current_order;
        insert_free_block(split_buddy);
    }

    block->next = NULL;
    block->is_free = 0;
    block->requested_size = size;

    allocator.stats.total_allocated += size;
    allocator.stats.current_usage += size;
    allocator.stats.allocation_count++;
    allocator.stats.allocated_blocks++;
    if (allocator.stats.current_usage > allocator.stats.peak_usage) {
        allocator.stats.peak_usage = allocator.stats.current_usage;
    }

    return (void *)(block + 1);
}

void buddy_free(void *ptr) {
    buddy_block_t *block;

    if (ptr == NULL || !allocator.initialized) {
        return;
    }

    block = ((buddy_block_t *)ptr) - 1;
    if (!block_in_heap(block) || block->is_free) {
        return;
    }

    allocator.stats.total_freed += block->requested_size;
    if (allocator.stats.current_usage >= block->requested_size) {
        allocator.stats.current_usage -= block->requested_size;
    } else {
        allocator.stats.current_usage = 0;
    }
    allocator.stats.deallocation_count++;
    if (allocator.stats.allocated_blocks > 0) {
        allocator.stats.allocated_blocks--;
    }

    block->requested_size = 0;
    block->is_free = 1;

    while (block->order < allocator.max_order) {
        buddy_block_t *buddy = buddy_for_block(block);

        if (buddy == NULL || !block_in_heap(buddy) ||
            !buddy->is_free || buddy->order != block->order) {
            break;
        }
        if (!remove_free_block(buddy, buddy->order)) {
            break;
        }

        if (buddy < block) {
            block = buddy;
        }
        block->order++;
        block->requested_size = 0;
        block->is_free = 1;
        block->next = NULL;
    }

    insert_free_block(block);
}

buddy_stats_t buddy_stats(void) {
    buddy_stats_t stats = allocator.stats;
    size_t total_free = 0;
    size_t largest_free = 0;
    uint64_t free_blocks = 0;

    stats.heap_size = allocator.heap_size;

    if (!allocator.initialized) {
        return stats;
    }

    for (unsigned int order = 0; order <= allocator.max_order; order++) {
        for (buddy_block_t *block = allocator.free_lists[order];
             block != NULL;
             block = block->next) {
            size_t size = block_size_for_order(order);

            free_blocks++;
            total_free += size;
            if (size > largest_free) {
                largest_free = size;
            }
        }
    }

    stats.free_blocks = free_blocks;
    stats.largest_free_block = largest_free;
    if (total_free == 0 || largest_free == total_free) {
        stats.fragmentation_ratio = 0.0;
    } else {
        stats.fragmentation_ratio = 1.0 - ((double)largest_free / (double)total_free);
    }

    return stats;
}
