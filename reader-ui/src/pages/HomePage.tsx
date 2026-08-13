import { FormEvent, useEffect, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { Input, Loading } from 'animal-island-ui';
import { listCategories, listFeaturedBooks, listSubCategories } from '../api/reader';
import { AppShell } from '../components/AppShell';
import { BookCategoryNav } from '../components/BookCategoryNav';
import { BookTextCard } from '../components/BookTextCard';
import { sampleCategories, sampleFeaturedBooks } from '../mocks/sampleBooks';
import type { ReaderBookSummary, ReaderCategory } from '../types/reader';
import { useSsrData } from '../seo/SsrDataContext';

export function HomePage() {
  const initial = useSsrData().home;
  const navigate = useNavigate();
  const [keyword, setKeyword] = useState('');
  const [featured, setFeatured] = useState<ReaderBookSummary[]>(initial?.featured || []);
  const [categories, setCategories] = useState<ReaderCategory[]>(initial?.categories || []);
  const [subCategories, setSubCategories] = useState<ReaderCategory[]>(initial?.subCategories || []);
  const [loading, setLoading] = useState(!initial);

  useEffect(() => {
    if (initial) return undefined;
    let alive = true;
    async function load() {
      setLoading(true);
      try {
        const [[featuredBooks, categoryRows], subCategoryRows] = await Promise.all([
          Promise.all([listFeaturedBooks(), listCategories()]),
          listSubCategories().catch(() => [])
        ]);
        if (!alive) {
          return;
        }
        setFeatured(featuredBooks.length ? featuredBooks : sampleFeaturedBooks);
        setCategories(categoryRows.length ? categoryRows : sampleCategories);
        setSubCategories(subCategoryRows);
      } catch {
        if (alive) {
          setFeatured(sampleFeaturedBooks);
          setCategories(sampleCategories);
        }
      } finally {
        if (alive) {
          setLoading(false);
        }
      }
    }
    load();
    return () => {
      alive = false;
    };
  }, [initial]);

  const submitSearch = (event: FormEvent) => {
    event.preventDefault();
    const query = keyword.trim();
    navigate(query ? `/books?keyword=${encodeURIComponent(query)}` : '/books');
  };

  return (
    <AppShell active="home" title="月白书城">
      <div className="home-sticky">
        <form className="search-band" onSubmit={submitSearch}>
          <Input
            size="large"
            value={keyword}
            placeholder="搜索书名、作者"
            onChange={(event) => setKeyword(event.target.value)}
            suffix={
              <button type="submit" className="search-band__go" aria-label="搜索">
                <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
                  <circle cx="11" cy="11" r="7" />
                  <path d="m20 20-3.2-3.2" />
                </svg>
              </button>
            }
          />
        </form>
        <BookCategoryNav
          categories={categories}
          subCategories={subCategories}
          onSelectCategory={(categoryCode) => navigate(categoryCode ? `/books?category=${encodeURIComponent(categoryCode)}` : '/books')}
          onSelectSubCategory={(subCategoryCode) => navigate(subCategoryCode ? `/books?subCategory=${encodeURIComponent(subCategoryCode)}` : '/books')}
        />
      </div>

      <main className="reader-page">
        <section className="content-section">
          <div className="section-head">
            <h2 className="section-title">精选作品</h2>
            <Link to="/books">全部</Link>
          </div>
          {loading ? <Loading /> : null}
          <div className="book-list">
            {featured.map((book) => (
              <BookTextCard key={book.bookId} book={book} to={`/books/${book.bookId}`} />
            ))}
          </div>
        </section>
      </main>
    </AppShell>
  );
}
