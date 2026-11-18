export function pageSlice(items, requestedPage, size = 20) {
  const pages = Math.max(1, Math.ceil(items.length / size));
  const page = Math.min(Math.max(1, requestedPage), pages);
  return { page, pages, items: items.slice((page - 1) * size, page * size) };
}
