'use client';

import { useEffect } from 'react';
import { useRouter } from 'next/navigation';

export default function Home() {
  const router = useRouter();

  useEffect(() => {
    const userId = localStorage.getItem('user_id');
    
    if (userId) {
      router.push('/applications');
    } else {
      router.push('/login');
    }
  }, [router]);

  return null;
}
