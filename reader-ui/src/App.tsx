import { Route, Routes } from 'react-router-dom';
import { BookDetailPage } from './pages/BookDetailPage';
import { BooksPage } from './pages/BooksPage';
import { BookshelfPage } from './pages/BookshelfPage';
import { FeedbackPage } from './pages/FeedbackPage';
import { HomePage } from './pages/HomePage';
import { LikedBooksPage } from './pages/LikedBooksPage';
import { LoginPage } from './pages/LoginPage';
import { MePage } from './pages/MePage';
import { ReaderPage } from './pages/ReaderPage';
import { RechargeOrderPage } from './pages/RechargeOrderPage';
import { RechargePage } from './pages/RechargePage';
import { RegisterPage } from './pages/RegisterPage';
import { SettingsPage } from './pages/SettingsPage';
import { WalletDetailPage } from './pages/WalletDetailPage';

export function App() {
  return (
    <Routes>
      <Route path="/" element={<HomePage />} />
      <Route path="/books" element={<BooksPage />} />
      <Route path="/shelf" element={<BookshelfPage />} />
      <Route path="/books/:bookId" element={<BookDetailPage />} />
      <Route path="/read/:bookId" element={<ReaderPage />} />
      <Route path="/auth/login" element={<LoginPage />} />
      <Route path="/auth/register" element={<RegisterPage />} />
      <Route path="/me" element={<MePage />} />
      <Route path="/me/likes" element={<LikedBooksPage />} />
      <Route path="/me/feedback" element={<FeedbackPage />} />
      <Route path="/me/settings" element={<SettingsPage />} />
      <Route path="/me/diamonds" element={<WalletDetailPage coinType="recharge" />} />
      <Route path="/me/recharge" element={<RechargePage />} />
      <Route path="/me/recharge/orders/:orderId" element={<RechargeOrderPage />} />
      <Route path="/me/coins" element={<WalletDetailPage coinType="bonus" />} />
    </Routes>
  );
}
