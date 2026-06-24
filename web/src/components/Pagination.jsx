export default function Pagination({ offset, limit, total, onChange }) {
  const totalPages = Math.max(1, Math.ceil(total / limit));
  const currentPage = Math.floor(offset / limit) + 1;

  if (totalPages <= 1) return null;

  const pages = [];
  const maxVisible = 5;
  let start = Math.max(1, currentPage - Math.floor(maxVisible / 2));
  let end = Math.min(totalPages, start + maxVisible - 1);
  if (end - start + 1 < maxVisible) {
    start = Math.max(1, end - maxVisible + 1);
  }

  for (let i = start; i <= end; i++) {
    pages.push(i);
  }

  const goToPage = (page) => {
    onChange((page - 1) * limit);
  };

  return (
    <div className="pagination">
      <button
        className="pagination-btn"
        disabled={currentPage <= 1}
        onClick={() => goToPage(currentPage - 1)}
      >
        ‹
      </button>

      {start > 1 && (
        <>
          <button className="pagination-btn" onClick={() => goToPage(1)}>1</button>
          {start > 2 && <span className="pagination-info">…</span>}
        </>
      )}

      {pages.map((page) => (
        <button
          key={page}
          className={`pagination-btn ${page === currentPage ? 'active' : ''}`}
          onClick={() => goToPage(page)}
        >
          {page}
        </button>
      ))}

      {end < totalPages && (
        <>
          {end < totalPages - 1 && <span className="pagination-info">…</span>}
          <button className="pagination-btn" onClick={() => goToPage(totalPages)}>
            {totalPages}
          </button>
        </>
      )}

      <button
        className="pagination-btn"
        disabled={currentPage >= totalPages}
        onClick={() => goToPage(currentPage + 1)}
      >
        ›
      </button>

      <span className="pagination-info">
        {total.toLocaleString()} results
      </span>
    </div>
  );
}
