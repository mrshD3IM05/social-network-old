'use client'

import { useMe } from '@/lib/useMe'
import usePaged from '@/lib/usePaged'
import Empty from '@/components/Empty'
import LoadMore from '@/components/LoadMore'
import PageHeader from '@/components/PageHeader'
import PostCard from '@/components/PostCard'
import PostForm from '@/components/PostForm'

export default function HomePage() {
  const { me } = useMe()
  const posts = usePaged('/posts')

  if (!me) return <p className="loading">Loading…</p>

  return (
    <>
      <PageHeader title={`Good to see you, ${me.first_name}.`} subtitle="The latest from you and the people you follow." />
      <PostForm onPosted={posts.reload} />
      {posts.error && <p className="error">{posts.error.message}</p>}
      {posts.items?.length === 0 && <Empty title="Nothing here yet">Write the first post, or follow people to fill your feed.</Empty>}
      {posts.items?.map(post => <PostCard key={post.id} post={post} myId={me.id} onDeleted={posts.reload} />)}
      <LoadMore list={posts} />
    </>
  )
}
