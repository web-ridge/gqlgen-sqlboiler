package auth

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/patrickmn/go-cache"
	"github.com/web-ridge/gqlgen-sqlboiler/examples/social-network/models"
	"github.com/web-ridge/utils-go/api"
	"golang.org/x/crypto/bcrypt"
)

const cacheSeparator = "|-._.-|"

// A private key for context that only this package can access. This is important
// to prevent collisions between different context uses
var userCtxKey = &contextKey{"user"}

type contextKey struct {
	name string
}

// Middleware decodes the share session cookie and packs the session into context
func Middleware(db *sql.DB) func(http.Handler) http.Handler {

	// We use a cache to prevent bcrypt using too much CPU
	// also this is 60ms faster if the user is cached ;)
	// Create a cache with a default expiration time of 5 minutes, and which
	// purges expired items every 10 minutes
	c := cache.New(5*time.Minute, 10*time.Minute)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			email, password, ok := r.BasicAuth()
			if !ok {
				next.ServeHTTP(w, r)
				return
			}

			// let's check our cache for this user
			cacheKey := email + cacheSeparator + password

			// get the user from cache
			cachedUser, userIsCached := c.Get(cacheKey)
			if userIsCached {
				// put it in context
				ctx := context.WithValue(r.Context(), userCtxKey, cachedUser)

				// and call the next with our new context
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// get user from database
			user, err := models.Users(
				models.UserWhere.Email.EQ(email),
			).One(r.Context(), db)
			if err != nil {
				api.WriteJSONError(w, "ss", "ss", http.StatusUnauthorized)
				return
			}

			// check if password is right
			if err = bcrypt.CompareHashAndPassword(user.Password, []byte(password)); err != nil {
				api.WriteJSONError(w, "dd", "dd", http.StatusUnauthorized)
				return
			}

			// put the user in cache so less cpu is used for checking the hash
			c.Set(cacheKey, user, cache.DefaultExpiration)

			// put the user in context so it can be used in resolvers
			ctx := context.WithValue(r.Context(), userCtxKey, user)

			// and call the next with our new context
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// FromContext finds the user from the context. REQUIRES Middleware to have run.
func FromContextWithCheck(ctx context.Context) (*models.User, bool) {
	user, exist := ctx.Value(userCtxKey).(*models.User)
	return user, exist
}

func ExistsInContext(ctx context.Context) bool {
	_, exist := ctx.Value(userCtxKey).(*models.User)
	return exist
}

// FromContext finds the user from the context. REQUIRES Middleware to have run.
func FromContext(ctx context.Context) *models.User {
	user, _ := ctx.Value(userCtxKey).(*models.User)
	return user
}

func UserIDFromContext(ctx context.Context) uint {
	user := FromContext(ctx)
	if user != nil {
		return user.ID
	}
	return 0
}
