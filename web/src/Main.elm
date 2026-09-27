module Main exposing (main)

import Api
import Browser
import Model exposing (Model, Msg(..), Tab(..))
import Time
import Update
import View.Layout


{-| The flag is the URL hash, which selects the starting tab.
-}
main : Program String Model Msg
main =
    Browser.element
        { init = init
        , update = Update.update
        , view = View.Layout.view
        , subscriptions = \_ -> Time.every 1000 Tick
        }


init : String -> ( Model, Cmd Msg )
init hash =
    let
        ( model, cmd ) =
            Update.update (SetTab (tabFromHash hash)) Model.init
    in
    ( model, Cmd.batch [ cmd, Api.fetchState, Api.fetchMachines, Api.fetchFonts ] )


tabFromHash : String -> Tab
tabFromHash hash =
    case hash of
        "#grid" ->
            GridTab

        "#label" ->
            LabelTab

        _ ->
            ControlTab
