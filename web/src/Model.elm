module Model exposing
    ( Field(..)
    , FontInfo
    , GridForm
    , Job
    , LabelForm
    , LaserForm
    , LineForm
    , MachineState
    , Offsets
    , PadMode(..)
    , Model
    , Msg(..)
    , Preview
    , Rect
    , RemoteData(..)
    , SeenMachine
    , Seg
    , Status
    , Tab(..)
    , Warning
    , init
    )

import Time


type Tab
    = ControlTab
    | LabelTab
    | GridTab


{-| What the jog pad moves: the machine head, or the work origin.
-}
type PadMode
    = MoveHead
    | MoveOrigin


type RemoteData a
    = NotAsked
    | Loading
    | Failure String
    | Success a


type alias Status =
    { state : String
    , wpos : List Float
    , mpos : List Float
    , tool : Int
    , laserMode : Bool
    , laserOn : Bool
    , playing : Bool
    , progress : List Int
    }


type alias Job =
    { name : String
    , phase : String
    , sent : Int
    , total : Int
    , error : Maybe String
    }


type alias MachineState =
    { connected : Bool
    , addr : String
    , error : Maybe String
    , status : Maybe Status
    , job : Maybe Job
    , log : List String
    , homed : Bool
    , halt : Maybe String
    , needsReset : Bool
    , originSet : Bool
    , offsets : Maybe Offsets
    , traced : Bool
    , zProbed : Bool
    , labelsDone : Int
    }


type alias SeenMachine =
    { name : String
    , addr : String
    , busy : Bool
    , state : String
    }


type alias Offsets =
    { x : Float
    , y : Float
    , z : Float
    }


type alias FontInfo =
    { id : String
    , family : String
    , style : String
    }


type alias Warning =
    { code : String
    , message : String
    }


type alias Rect =
    { minX : Float
    , minY : Float
    , maxX : Float
    , maxY : Float
    }


type alias Seg =
    { x1 : Float
    , y1 : Float
    , x2 : Float
    , y2 : Float
    }


type alias Preview =
    { burn : List Seg
    , travel : List Seg
    , bounds : Rect
    , workpiece : Maybe Rect
    , printable : Maybe Rect
    , warnings : List Warning
    , burnMM : Float
    , travelMM : Float
    , estimateSec : Float
    , scale : Float
    , lines : Int
    , gcode : String
    }


{-| Powers are percentages, one per row (bottom first); feeds are mm/min,
one per column (left first).
-}
type alias GridForm =
    { powers : List String
    , feeds : List String
    , cell : String
    , gap : String
    , interval : String
    }


type alias LineForm =
    { text : String
    , size : String
    }


type alias LabelForm =
    { lines : List LineForm
    , fontID : String
    , fontFilter : String
    , align : String
    , spacing : String
    , fit : Bool
    , width : String
    , height : String
    , margin : String
    }


type alias LaserForm =
    { power : String
    , feed : String
    , passes : String
    , mode : String
    , interval : String
    , angle : String
    , bidir : Bool
    }


{-| Form fields that are edited as plain text.
-}
type Field
    = GridCell
    | GridGap
    | GridInterval
    | LabelFontFilter
    | LabelAlign
    | LabelSpacing
    | LabelWidth
    | LabelHeight
    | LabelMargin
    | LaserPower
    | LaserFeed
    | LaserPasses
    | LaserMode
    | LaserInterval
    | LaserAngle


type alias Model =
    { tab : Tab
    , machine : Maybe MachineState
    , host : String
    , console : String
    , padMode : PadMode
    , jogStep : Float
    , nudgeStep : Float
    , retrace : Bool
    , pointer : Bool
    , framing : String
    , grid : GridForm
    , gridPreview : RemoteData Preview
    , label : LabelForm
    , laser : LaserForm
    , labelPreview : RemoteData Preview
    , previewSeq : Int
    , fonts : RemoteData (List FontInfo)
    , showTravel : Bool
    , showGcode : Bool
    , notice : Maybe String
    , noticeSeq : Int
    , machines : List SeenMachine
    }


type Msg
    = SetTab Tab
    | Tick Time.Posix
    | GotState (Result String MachineState)
    | GotAction (Result String MachineState)
    | GotFonts (Result String (List FontInfo))
    | SetHost String
    | Connect
    | ConnectTo String
    | GotMachines (Result String (List SeenMachine))
    | Disconnect
    | SetConsole String
    | SendConsole
    | GotCommand (Result String (List String))
    | SetPadMode PadMode
    | SetPadStep Float
    | Pad String Float
    | ToggleRetrace
    | SetPointer Bool
    | ZeroAxes String
    | ResetCounter
    | GoTo String
    | Abort
    | Hold
    | Resume
    | SetField Field String
    | SetLineText Int String
    | SetLineSize Int String
    | AddLine
    | RemoveLine Int
    | SetFont String
    | ToggleBidir
    | ToggleFit
    | SetFraming String
    | GotPreview Tab Int (Result String Preview)
    | RunJob Tab
    | ToggleTravel
    | ToggleGcode
    | DismissNotice
    | ExpireNotice Int
    | Home
    | Unlock
    | Reset
    | SetTool String
    | AcceptOrigin
    | Probe Bool Bool
    | SetGridPower Int String
    | AddGridPower
    | RemoveGridPower Int
    | SetGridFeed Int String
    | AddGridFeed
    | RemoveGridFeed Int


init : Model
init =
    { tab = ControlTab
    , machine = Nothing
    , host = ""
    , console = ""
    , padMode = MoveHead
    , jogStep = 1
    , nudgeStep = 0.5
    , retrace = True
    , pointer = False
    , framing = "full"
    , grid =
        { powers = [ "20", "30", "40", "50", "60" ]
        , feeds = [ "500", "1000", "1500", "2000" ]
        , cell = "5"
        , gap = "2"
        , interval = "0.1"
        }
    , gridPreview = NotAsked
    , label =
        { lines = [ { text = "Blåbær", size = "8" }, { text = "2026", size = "6" } ]
        , fontID = ""
        , fontFilter = ""
        , align = "center"
        , spacing = "1.3"
        , fit = False
        , width = "50"
        , height = "50"
        , margin = "4"
        }
    , laser =
        { power = "70"
        , feed = "2800"
        , passes = "1"
        , mode = "fill"
        , interval = "0.1"
        , angle = "0"
        , bidir = True
        }
    , labelPreview = NotAsked
    , previewSeq = 0
    , fonts = Loading
    , showTravel = False
    , showGcode = False
    , notice = Nothing
    , noticeSeq = 0
    , machines = []
    }
